package graph

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	tanglePackage "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

const (
	isSyncThreshold = 1
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

	broadcast = make(chan *wsMessage, 100)

	wasSyncBefore = false

	server *http.Server
	router *http.ServeMux
)

// PageData struct for html template
type PageData struct {
	URI string
}

func wrapHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" || r.URL.Path == "/index.htm" {
			data := PageData{
				URI: parameter.NodeConfig.GetString("graph.websocket.uri"),
			}
			tmpl, _ := template.New("graph").Parse(index)
			tmpl.Execute(w, data)
			return
		}
		h.ServeHTTP(w, r)
	}
}

func socketServer(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("Upgrade websocket:", err)
		return
	}
	// register client
	clientsLock.Lock()
	clients[c] = true
	clientsLock.Unlock()
	onConnect(c)
}

func socketBroadcast() {
	for message := range broadcast {
		clientsLock.Lock()
		for client := range clients {
			err := client.WriteJSON(message)
			if err != nil {
				log.Warnf("Websocket error: %s", err)
				client.Close()
				delete(clients, client)
				log.Infof("Removed dead websocket client")
			}
		}
		clientsLock.Unlock()
	}
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	initRingBuffers()

	router = http.NewServeMux()

	// websocket and web server
	server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", parameter.NodeConfig.GetString("graph.bindAddress"), parameter.NodeConfig.GetInt("graph.port")),
		Handler: router,
	}

	fs := http.FileServer(http.Dir(parameter.NodeConfig.GetString("graph.webrootPath")))

	router.HandleFunc("/", wrapHandler(fs))
	router.HandleFunc("/ws", socketServer)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*tanglePackage.CachedTransaction)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*tanglePackage.CachedTransaction), task.Param(1).(milestone_index.MilestoneIndex), task.Param(2).(int64)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewMilestone(task.Param(0).(*tanglePackage.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newMilestoneWorkerCount), workerpool.QueueSize(newMilestoneWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))
}

func run(plugin *node.Plugin) {

	notifyNewTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSynced() || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) {
				// Not sync
				cachedTx.Release(true) // tx -1
				return
			}
			wasSyncBefore = true
		}

		if (firstSeenLatestMilestoneIndex - latestSolidMilestoneIndex) <= isSyncThreshold {
			_, added := newTxWorkerPool.TrySubmit(cachedTx) // tx pass +1
			if added {
				return // Avoid tx -1 (done inside workerpool task)
			}
		}
		cachedTx.Release(true) // tx -1
	})

	notifyConfirmedTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if wasSyncBefore {
			_, added := confirmedTxWorkerPool.TrySubmit(cachedTx, msIndex, confTime) // tx pass +1
			if added {
				return // Avoid tx -1 (done inside workerpool task)
			}
		}
		cachedTx.Release(true) // tx -1
	})

	notifyNewMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if wasSyncBefore {
			_, added := newMilestoneWorkerPool.TrySubmit(cachedBndl) // bundle pass +1
			if added {
				return // Avoid bundle -1 (done inside workerpool task)
			}
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
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Graph[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping Graph[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Graph[NewMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[NewMilestoneWorker] ... done")
		tangle.Events.ReceivedNewMilestone.Attach(notifyNewMilestone)
		newMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.ReceivedNewMilestone.Detach(notifyNewMilestone)
		newMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping Graph[NewMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Graph Webserver", func(shutdownSignal <-chan struct{}) {

		go func() {
			if err := server.ListenAndServe(); (err != nil) && (err != http.ErrServerClosed) {
				log.Error(err.Error())
			}
		}()

		go socketBroadcast()

		log.Infof("You can now access IOTA Tangle Visualiser using: http://%s:%d", parameter.NodeConfig.GetString("graph.bindAddress"), parameter.NodeConfig.GetInt("graph.port"))

		<-shutdownSignal
		log.Info("Stopping Graph ...")

		close(broadcast)

		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		log.Info("Stopping Graph ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}
