package graph

import (
	"html/template"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/websockethub"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglePackage "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("Graph", node.Disabled, configure, run)

	log *logger.Logger

	newTxWorkerCount     = 1
	newTxWorkerQueueSize = 10000
	newTxWorkerPool      *workerpool.WorkerPool

	confirmedTxWorkerCount     = 1
	confirmedTxWorkerQueueSize = 10000
	confirmedTxWorkerPool      *workerpool.WorkerPool

	newMilestoneWorkerCount     = 1
	newMilestoneWorkerQueueSize = 100
	newMilestoneWorkerPool      *workerpool.WorkerPool

	wasSyncBefore = false

	webSocketWriteTimeout = time.Duration(3) * time.Second

	router   *http.ServeMux
	server   *http.Server
	upgrader *websocket.Upgrader
	hub      *websockethub.Hub
)

// PageData struct for html template
type PageData struct {
	URI                string
	ExplorerTxLink     string
	ExplorerBundleLink string
}

func wrapHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" || r.URL.Path == "/index.htm" {
			data := PageData{
				URI:                config.NodeConfig.GetString(config.CfgGraphWebSocketURI),
				ExplorerTxLink:     config.NodeConfig.GetString(config.CfgGraphExplorerTxLink),
				ExplorerBundleLink: config.NodeConfig.GetString(config.CfgGraphExplorerBundleLink),
			}
			tmpl, _ := template.New("graph").Parse(index)
			tmpl.Execute(w, data)
			return
		}
		h.ServeHTTP(w, r)
	}
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	initRingBuffers()

	router = http.NewServeMux()

	// websocket and web server
	bindAddr := config.NodeConfig.GetString(config.CfgGraphBindAddress)
	server = &http.Server{Addr: bindAddr, Handler: router}

	upgrader = &websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		CheckOrigin:       func(r *http.Request) bool { return true }, // allow any origin for websocket connections
		EnableCompression: true,
	}

	hub = websockethub.NewHub(log, upgrader, broadcastQueueSize, clientSendChannelSize)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*tanglePackage.CachedTransaction)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*tanglePackage.CachedTransaction), task.Param(1).(milestone.Index), task.Param(2).(int64)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewMilestone(task.Param(0).(*tanglePackage.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newMilestoneWorkerCount), workerpool.QueueSize(newMilestoneWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))
}

func run(_ *node.Plugin) {

	notifyNewTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSyncedWithThreshold() {
				cachedTx.Release(true) // tx -1
				return
			}
			wasSyncBefore = true
		}

		if _, added := newTxWorkerPool.TrySubmit(cachedTx); added { // tx pass +1
			return // Avoid tx -1 (done inside workerpool task)
		}
		cachedTx.Release(true) // tx -1
	})

	notifyConfirmedTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, msIndex milestone.Index, confTime int64) {
		if !wasSyncBefore {
			cachedTx.Release(true) // tx -1
			return
		}

		if _, added := confirmedTxWorkerPool.TrySubmit(cachedTx, msIndex, confTime); added { // tx pass +1
			return // Avoid tx -1 (done inside workerpool task)
		}
		cachedTx.Release(true) // tx -1
	})

	notifyNewMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if !wasSyncBefore {
			cachedBndl.Release(true) // tx -1
			return
		}

		if _, added := newMilestoneWorkerPool.TrySubmit(cachedBndl); added { // bundle pass +1
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	daemon.BackgroundWorker("Graph[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		newTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping Graph[NewTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("Graph[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping Graph[ConfirmedTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("Graph[NewMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[NewMilestoneWorker] ... done")
		tangle.Events.ReceivedNewMilestone.Attach(notifyNewMilestone)
		newMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.ReceivedNewMilestone.Detach(notifyNewMilestone)
		newMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping Graph[NewMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("Graph Webserver", func(shutdownSignal <-chan struct{}) {

		go func() {
			if err := server.ListenAndServe(); (err != nil) && (err != http.ErrServerClosed) {
				log.Error(err.Error())
			}
		}()

		go hub.Run(shutdownSignal)

		router.HandleFunc("/", wrapHandler(http.FileServer(http.Dir(config.NodeConfig.GetString(config.CfgGraphWebRootPath)))))
		router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			hub.ServeWebsocket(w, r, func(client *websockethub.Client) {
				log.Info("WebSocket client connection established")

				config := &wsConfig{NetworkName: config.NodeConfig.GetString(config.CfgGraphNetworkName)}

				var initTxs []*wsTransaction
				txRingBufferLock.Lock()
				txRingBuffer.Do(func(tx interface{}) {
					if tx != nil {
						initTxs = append(initTxs, tx.(*wsTransaction))
					}
				})
				txRingBufferLock.Unlock()

				var initSns []*wsTransactionSn
				snRingBufferLock.Lock()
				snRingBuffer.Do(func(sn interface{}) {
					if sn != nil {
						initSns = append(initSns, sn.(*wsTransactionSn))
					}
				})
				snRingBufferLock.Unlock()

				var initMs []string
				msRingBufferLock.Lock()
				msRingBuffer.Do(func(ms interface{}) {
					if ms != nil {
						initMs = append(initMs, ms.(string))
					}
				})
				msRingBufferLock.Unlock()

				client.Send(&wsMessage{Type: "config", Data: config})
				client.Send(&wsMessage{Type: "inittx", Data: initTxs})
				client.Send(&wsMessage{Type: "initsn", Data: initSns})
				client.Send(&wsMessage{Type: "initms", Data: initMs})
			})
		})

		bindAddr := config.NodeConfig.GetString(config.CfgGraphBindAddress)
		log.Infof("You can now access IOTA Tangle Visualiser using: http://%s", bindAddr)

		<-shutdownSignal
		log.Info("Stopping Graph ...")

		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		log.Info("Stopping Graph ... done")
	}, shutdown.PriorityMetricsPublishers)
}
