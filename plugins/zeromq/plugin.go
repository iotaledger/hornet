package zeromq

import (
	"time"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/gohornet/hornet/packages/logger"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	tanglePackage "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/node"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/timeutil"
	"github.com/gohornet/hornet/packages/workerpool"
	"github.com/gohornet/hornet/plugins/tangle"
)

const (
	isSyncThreshold = 2
)

// PLUGIN ZeroMQ
var (
	PLUGIN = node.NewPlugin("ZeroMQ", node.Disabled, configure, run)
	log    = logger.NewLogger("ZeroMQ")

	newTxWorkerCount     = 1
	newTxWorkerQueueSize = 10000
	newTxWorkerPool      *workerpool.WorkerPool

	confirmedTxWorkerCount     = 1
	confirmedTxWorkerQueueSize = 10000
	confirmedTxWorkerPool      *workerpool.WorkerPool

	newLatestMilestoneWorkerCount     = 1
	newLatestMilestoneWorkerQueueSize = 100
	newLatestMilestoneWorkerPool      *workerpool.WorkerPool

	newSolidMilestoneWorkerCount     = 1
	newSolidMilestoneWorkerQueueSize = 100
	newSolidMilestoneWorkerPool      *workerpool.WorkerPool

	wasSyncBefore = false

	publisher *Publisher
)

// Configure the zeromq plugin
func configure(plugin *node.Plugin) {

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*hornet.Transaction))
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*hornet.Transaction), task.Param(1).(milestone_index.MilestoneIndex), task.Param(2).(int64))
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize))

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewLatestMilestone(task.Param(0).(*tanglePackage.Bundle))
		task.Return(nil)
	}, workerpool.WorkerCount(newLatestMilestoneWorkerCount), workerpool.QueueSize(newLatestMilestoneWorkerQueueSize))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewSolidMilestone(task.Param(0).(*tanglePackage.Bundle))
		task.Return(nil)
	}, workerpool.WorkerCount(newSolidMilestoneWorkerCount), workerpool.QueueSize(newSolidMilestoneWorkerQueueSize))
}

// Start the zeromq plugin
func run(plugin *node.Plugin) {

	log.Infof("Starting ZeroMQ Publisher (port %d) ...", parameter.NodeConfig.GetInt("zmq.port"))

	notifyNewTx := events.NewClosure(func(transaction *hornet.Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if (firstSeenLatestMilestoneIndex == 0) || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) || ((firstSeenLatestMilestoneIndex - latestSolidMilestoneIndex) > isSyncThreshold) {
				// Not sync
				return
			}
			wasSyncBefore = true
		}

		newTxWorkerPool.TrySubmit(transaction)
	})

	notifyConfirmedTx := events.NewClosure(func(transaction *hornet.Transaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if !wasSyncBefore {
			return
		}

		confirmedTxWorkerPool.TrySubmit(transaction, msIndex, confTime)
	})

	notifyNewLatestMilestone := events.NewClosure(func(bundle *tanglePackage.Bundle) {
		if !wasSyncBefore {
			return
		}

		newLatestMilestoneWorkerPool.TrySubmit(bundle)
	})

	notifyNewSolidMilestone := events.NewClosure(func(bundle *tanglePackage.Bundle) {
		if !wasSyncBefore {
			return
		}

		newSolidMilestoneWorkerPool.TrySubmit(bundle)
	})

	daemon.BackgroundWorker("ZeroMQ Publisher", func(shutdownSignal <-chan struct{}) {
		log.Infof("Starting ZeroMQ Publisher (port %d)", parameter.NodeConfig.GetInt("zmq.port"))

		go startPublisher()

		<-shutdownSignal
		log.Info("Stopping ZeroMQ Publisher ...")

		if err := publisher.Shutdown(); err != nil {
			log.Errorf("Stopping ZeroMQ Publisher: %s", err.Error())
		} else {
			log.Info("Stopping ZeroMQ Publisher ... done")
		}
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ address topic updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(updateAddressTopics, 5*time.Second, shutdownSignal)
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		newTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewSolidMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}

// Start the zmq publisher.
func startPublisher() error {
	pub, err := NewPublisher()
	if err != nil {
		return err
	}
	publisher = pub

	return publisher.Start(parameter.NodeConfig.GetInt("zmq.port"))
}
