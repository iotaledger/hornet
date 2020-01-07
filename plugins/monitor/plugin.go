package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	engineio "github.com/googollee/go-engine.io"
	"github.com/googollee/go-engine.io/transport"
	"github.com/googollee/go-engine.io/transport/polling"
	"github.com/googollee/go-engine.io/transport/websocket"
	socketio "github.com/googollee/go-socket.io"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/trinary"

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

	server            *http.Server
	apiServer         *http.Server
	router            *http.ServeMux
	socketioServer    *socketio.Server
	api               *gin.Engine
	tanglemonitorPath string
)

func configureSocketIOServer() error {
	var err error

	socketioServer, err = socketio.NewServer(&engineio.Options{
		PingTimeout:  time.Second * 20,
		PingInterval: time.Second * 5,
		Transports: []transport.Transport{
			polling.Default,
			websocket.Default,
		},
	})
	if err != nil {
		return err
	}

	socketioServer.OnConnect("/", onConnectHandler)
	socketioServer.OnError("/", onErrorHandler)
	socketioServer.OnDisconnect("/", onDisconnectHandler)

	return nil
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Monitor")

	initRingBuffer()

	gin.SetMode(gin.ReleaseMode)
	api = gin.New()
	// Recover from any panics and write a 500 if there was one
	api.Use(gin.Recovery())

	router = http.NewServeMux()
	tanglemonitorPath = parameter.NodeConfig.GetString("monitor.TangleMonitorPath")
	if tanglemonitorPath == "" {
		log.Panic("Tanglemonitor Path is empty")
	}

	fs := http.FileServer(http.Dir(tanglemonitorPath))

	err := configureSocketIOServer()
	if err != nil {
		log.Panic(err.Error())
	}

	router.Handle("/", fs)
	router.Handle("/socket.io/", socketioServer)

	api.GET("/api/v1/getRecentTransactions", handleAPI)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		tx := task.Param(0).(*tanglePackage.CachedTransaction)
		onNewTx(tx)
		tx.Release()
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		tx := task.Param(0).(*tanglePackage.CachedTransaction)
		onConfirmedTx(tx, task.Param(1).(milestone_index.MilestoneIndex), task.Param(2).(int64))
		tx.Release()
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize))

	newMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewMilestone(task.Param(0).(*tanglePackage.Bundle))
		task.Return(nil)
	}, workerpool.WorkerCount(newMilestoneWorkerCount), workerpool.QueueSize(newMilestoneWorkerQueueSize))

	reattachmentWorkerPool = workerpool.New(func(task workerpool.Task) {
		onReattachment(task.Param(0).(trinary.Hash))
		task.Return(nil)
	}, workerpool.WorkerCount(reattachmentWorkerCount), workerpool.QueueSize(reattachmentWorkerQueueSize))

}

func run(plugin *node.Plugin) {

	notifyNewTx := events.NewClosure(func(transaction *tanglePackage.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSynced() || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) {
				// Not sync
				return
			}
			wasSyncBefore = true
		}

		if (firstSeenLatestMilestoneIndex - latestSolidMilestoneIndex) <= isSyncThreshold {
			transaction.RegisterConsumer()
			_, added := newTxWorkerPool.TrySubmit(transaction)
			if !added {
				transaction.Release()
			}
		}
	})

	notifyConfirmedTx := events.NewClosure(func(transaction *tanglePackage.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if !wasSyncBefore {
			return
		}

		transaction.RegisterConsumer()
		_, added := confirmedTxWorkerPool.TrySubmit(transaction, msIndex, confTime)
		if !added {
			transaction.Release()
		}
	})

	notifyNewMilestone := events.NewClosure(func(bundle *tanglePackage.Bundle) {
		if !wasSyncBefore {
			return
		}

		newMilestoneWorkerPool.TrySubmit(bundle)
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
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[NewMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[NewMilestoneWorker] ... done")
		tangle.Events.ReceivedNewMilestone.Attach(notifyNewMilestone)
		newMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[NewMilestoneWorker] ...")
		tangle.Events.ReceivedNewMilestone.Detach(notifyNewMilestone)
		newMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[NewMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor[ReattachmentWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Monitor[ReattachmentWorker] ... done")
		reattachmentWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Monitor[ReattachmentWorker] ...")
		reattachmentWorkerPool.StopAndWait()
		log.Info("Stopping Monitor[ReattachmentWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("Monitor Webserver", func(shutdownSignal <-chan struct{}) {

		// socket.io and web server
		server = &http.Server{
			Addr:    fmt.Sprintf("%s:%d", parameter.NodeConfig.GetString("monitor.host"), parameter.NodeConfig.GetInt("monitor.port")),
			Handler: router,
		}

		// REST api server
		apiServer = &http.Server{
			Addr:    fmt.Sprintf("%s:%d", parameter.NodeConfig.GetString("monitor.host"), parameter.NodeConfig.GetInt("monitor.apiPort")),
			Handler: api,
		}

		go socketioServer.Serve()

		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Error(err.Error())
			}
		}()

		go func() {
			if err := apiServer.ListenAndServe(); err != nil {
				log.Errorf(err.Error())
			}
		}()

		log.Infof("You can now access TangleMonitor using: http://%s:%d", parameter.NodeConfig.GetString("monitor.host"), parameter.NodeConfig.GetInt("monitor.port"))

		<-shutdownSignal
		log.Info("Stopping Monitor ...")

		socketioServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		_ = apiServer.Shutdown(ctx)
		log.Info("Stopping Monitor ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}
