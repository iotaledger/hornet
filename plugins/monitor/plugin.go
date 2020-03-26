package monitor

import (
	"context"
	"html/template"
	"net/http"
	texttemp "text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/websockethub"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	tanglePackage "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

const (
	TX_BUFFER_SIZE       = 50000
	BROADCAST_QUEUE_SIZE = 20000
	isSyncThreshold      = 1
)

var (
	PLUGIN = node.NewPlugin("Monitor", node.Disabled, configure, run)
	log    *logger.Logger

	newTxWorkerPool        = (&async.NonBlockingWorkerPool{}).Tune(1)
	confirmedTxWorkerPool  = (&async.NonBlockingWorkerPool{}).Tune(1)
	newMilestoneWorkerPool = (&async.NonBlockingWorkerPool{}).Tune(1)
	reattachmentWorkerPool = (&async.NonBlockingWorkerPool{}).Tune(1)

	wasSyncBefore = false

	server            *http.Server
	apiServer         *http.Server
	router            *http.ServeMux
	api               *gin.Engine
	tanglemonitorPath string
	upgrader          *websocket.Upgrader
	hub               *websockethub.Hub
)

type PageData struct {
	WebsocketURI string
	APIPort      string
	InitTxAmount int
}

func wrapHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" || r.URL.Path == "/index.htm" {
			data := PageData{
				WebsocketURI: config.NodeConfig.GetString(config.CfgMonitorWebSocketURI),
				APIPort:      config.NodeConfig.GetString(config.CfgMonitorRemoteAPIPort),
				InitTxAmount: config.NodeConfig.GetInt(config.CfgMonitorInitialTransactionsCount),
			}
			tmpl, _ := template.New("monitorIndex").Parse(index)
			tmpl.Execute(w, data)
			return
		} else if r.URL.Path == "/js/tangleview.mod.js" {
			tmpl, _ := texttemp.New("monitorJS").Parse(tangleviewJS)
			tmpl.Execute(w, nil)
			return
		}
		h.ServeHTTP(w, r)
	}
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	initRingBuffer()

	gin.SetMode(gin.ReleaseMode)
	api = gin.New()
	// Recover from any panics and write a 500 if there was one
	api.Use(gin.Recovery())

	router = http.NewServeMux()
	tanglemonitorPath = config.NodeConfig.GetString(config.CfgMonitorTangleMonitorPath)
	if tanglemonitorPath == "" {
		log.Panic("Tanglemonitor Path is empty")
	}

	upgrader = &websocket.Upgrader{
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
	}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	hub = websockethub.NewHub(log, upgrader, BROADCAST_QUEUE_SIZE)

	api.GET("/api/v1/getRecentTransactions", handleAPI)
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
			if added := newTxWorkerPool.Submit(func() { onNewTx(cachedTx) }); added { // tx pass +1
				return // Avoid tx -1 (done inside workerpool task)
			}
		}
		cachedTx.Release(true) // tx -1
	})

	notifyConfirmedTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if wasSyncBefore {
			if added := confirmedTxWorkerPool.Submit(func() { onConfirmedTx(cachedTx, msIndex, confTime) }); added { // tx pass +1
				return // Avoid tx -1 (done inside workerpool task)
			}
		}
		cachedTx.Release(true) // tx -1
	})

	notifyNewMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if wasSyncBefore {
			if added := newMilestoneWorkerPool.Submit(func() { onNewMilestone(cachedBndl) }); added { // bundle pass +1
				return // Avoid bundle -1 (done inside workerpool task)
			}
		}
		cachedBndl.Release(true) // bundle -1
	})

	daemon.BackgroundWorker("Monitor[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		<-shutdownSignal
		log.Info("Stopping Monitor[NewTxWorker] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.ShutdownGracefully()
		log.Info("Stopping Monitor[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		<-shutdownSignal
		log.Info("Stopping Monitor[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.ShutdownGracefully()
		log.Info("Stopping Monitor[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[NewMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[NewMilestoneWorker] ... done")
		tangle.Events.ReceivedNewMilestone.Attach(notifyNewMilestone)
		<-shutdownSignal
		log.Info("Stopping Monitor[NewMilestoneWorker] ...")
		tangle.Events.ReceivedNewMilestone.Detach(notifyNewMilestone)
		newMilestoneWorkerPool.ShutdownGracefully()
		log.Info("Stopping Monitor[NewMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[ReattachmentWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[ReattachmentWorker] ... done")
		<-shutdownSignal
		log.Info("Stopping Monitor[ReattachmentWorker] ...")
		reattachmentWorkerPool.Shutdown()
		log.Info("Stopping Monitor[ReattachmentWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor Webserver", func(shutdownSignal <-chan struct{}) {

		// Websocket and web server
		webBindAddr := config.NodeConfig.GetString(config.CfgMonitorWebBindAddress)
		server = &http.Server{Addr: webBindAddr, Handler: router}

		// REST api server
		apiBindAddr := config.NodeConfig.GetString(config.CfgMonitorAPIBindAddress)
		apiServer = &http.Server{Addr: apiBindAddr, Handler: api}

		go func() {
			if err := server.ListenAndServe(); (err != nil) && (err != http.ErrServerClosed) {
				log.Error(err.Error())
			}
		}()

		go func() {
			if err := apiServer.ListenAndServe(); (err != nil) && (err != http.ErrServerClosed) {
				log.Errorf(err.Error())
			}
		}()

		go hub.Run(shutdownSignal)

		router.HandleFunc("/", wrapHandler(http.FileServer(http.Dir(config.NodeConfig.GetString(config.CfgMonitorTangleMonitorPath)))))
		router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			hub.ServeWebsocket(w, r)
		})

		log.Infof("You can now access TangleMonitor using: http://%s", webBindAddr)

		<-shutdownSignal
		log.Info("Stopping Monitor ...")

		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		_ = apiServer.Shutdown(ctx)
		log.Info("Stopping Monitor ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}
