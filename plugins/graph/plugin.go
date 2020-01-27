package graph

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/context"

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

	wasSyncBefore = false

	server         *http.Server
	router         *http.ServeMux
	socketioServer *socketio.Server
)

func downloadSocketIOHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, parameter.NodeConfig.GetString("graph.socketioPath"))
}

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
	log = logger.NewLogger(plugin.Name)
	initRingBuffers()

	router = http.NewServeMux()

	// socket.io and web server
	server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", parameter.NodeConfig.GetString("graph.bindAddress"), parameter.NodeConfig.GetInt("graph.port")),
		Handler: router,
	}

	fs := http.FileServer(http.Dir(parameter.NodeConfig.GetString("graph.webrootPath")))

	err := configureSocketIOServer()
	if err != nil {
		log.Panicf("Graph: %v", err.Error())
	}

	router.Handle("/", fs)
	router.HandleFunc("/socket.io/socket.io.js", downloadSocketIOHandler)
	router.Handle("/socket.io/", socketioServer)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*tanglePackage.CachedTransaction)) //Pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*tanglePackage.CachedTransaction), task.Param(1).(milestone_index.MilestoneIndex), task.Param(2).(int64)) //Pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize))

	newMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewMilestone(task.Param(0).(*tanglePackage.Bundle))
		task.Return(nil)
	}, workerpool.WorkerCount(newMilestoneWorkerCount), workerpool.QueueSize(newMilestoneWorkerQueueSize))

}

func run(plugin *node.Plugin) {

	notifyNewTx := events.NewClosure(func(transaction *tanglePackage.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSynced() || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) {
				// Not sync
				transaction.Release() //-1
				return
			}
			wasSyncBefore = true
		}

		if (firstSeenLatestMilestoneIndex - latestSolidMilestoneIndex) <= isSyncThreshold {
			_, added := newTxWorkerPool.TrySubmit(transaction) //Pass +1
			if added {
				return // Avoid Release()
			}
		}
		transaction.Release() //-1
	})

	notifyConfirmedTx := events.NewClosure(func(transaction *tanglePackage.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if wasSyncBefore {
			_, added := confirmedTxWorkerPool.TrySubmit(transaction, msIndex, confTime) //Pass +1
			if added {
				return // Avoid Release()
			}
		}
		transaction.Release() //-1
	})

	notifyNewMilestone := events.NewClosure(func(bundle *tanglePackage.Bundle) {
		if !wasSyncBefore {
			return
		}

		newMilestoneWorkerPool.TrySubmit(bundle)
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
		go socketioServer.Serve()

		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Error(err.Error())
			}
		}()

		log.Infof("You can now access IOTA Tangle Visualiser using: http://%s:%d", parameter.NodeConfig.GetString("graph.bindAddress"), parameter.NodeConfig.GetInt("graph.port"))

		<-shutdownSignal
		log.Info("Stopping Graph ...")

		socketioServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		log.Info("Stopping Graph ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}
