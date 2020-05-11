package monitor

import (
	"context"
	"html/template"
	"net/http"
	texttemp "text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/websockethub"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglePackage "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

const (
	txBufferSize          = 50000
	broadcastQueueSize    = 20000
	clientSendChannelSize = 1000
)

var (
	PLUGIN = node.NewPlugin("Monitor", node.Disabled, configure, run)
	log    *logger.Logger

	newTxWorkerCount     = 1
	newTxWorkerQueueSize = 10000
	newTxWorkerPool      *workerpool.WorkerPool

	confirmedTxWorkerCount     = 1
	confirmedTxWorkerQueueSize = 10000
	confirmedTxWorkerPool      *workerpool.WorkerPool

	newMilestoneWorkerCount     = 1
	newMilestoneWorkerQueueSize = 100
	newMilestoneWorkerPool      *workerpool.WorkerPool

	reattachmentWorkerCount     = 1
	reattachmentWorkerQueueSize = 100
	reattachmentWorkerPool      *workerpool.WorkerPool

	wasSyncBefore = false

	webSocketWriteTimeout = time.Duration(3) * time.Second

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
				InitTxAmount: config.NodeConfig.GetInt(config.CfgMonitorInitialTransactions),
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
		HandshakeTimeout:  webSocketWriteTimeout,
		CheckOrigin:       func(r *http.Request) bool { return true }, // allow any origin for websocket connections
		EnableCompression: true,
	}

	hub = websockethub.NewHub(log, upgrader, broadcastQueueSize, clientSendChannelSize)

	api.GET("/api/v1/getRecentTransactions", handleAPI)

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

	reattachmentWorkerPool = workerpool.New(func(task workerpool.Task) {
		onReattachment(task.Param(0).(trinary.Hash))
		task.Return(nil)
	}, workerpool.WorkerCount(reattachmentWorkerCount), workerpool.QueueSize(reattachmentWorkerQueueSize))

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
			// Not sync
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
			// Not sync
			cachedBndl.Release(true) // tx -1
			return
		}

		if _, added := newMilestoneWorkerPool.TrySubmit(cachedBndl); added { // bundle pass +1
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	daemon.BackgroundWorker("Monitor[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		newTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[NewTxWorker] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[NewTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[ConfirmedTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[NewMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[NewMilestoneWorker] ... done")
		tangle.Events.ReceivedNewMilestone.Attach(notifyNewMilestone)
		newMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[NewMilestoneWorker] ...")
		tangle.Events.ReceivedNewMilestone.Detach(notifyNewMilestone)
		newMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[NewMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[ReattachmentWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[ReattachmentWorker] ... done")
		reattachmentWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[ReattachmentWorker] ...")
		reattachmentWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[ReattachmentWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

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
	}, shutdown.PriorityMetricsPublishers)
}
