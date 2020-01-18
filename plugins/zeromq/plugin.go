package zeromq

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
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

// PLUGIN ZeroMQ
var (
	PLUGIN = node.NewPlugin("ZeroMQ", node.Disabled, configure, run)
	log    *logger.Logger

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
	log = logger.NewLogger(plugin.Name)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		tx := task.Param(0).(*tanglePackage.CachedTransaction) //1
		onNewTx(tx)
		tx.Release() //-1
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		tx := task.Param(0).(*tanglePackage.CachedTransaction) //1
		onConfirmedTx(tx, task.Param(1).(milestone_index.MilestoneIndex), task.Param(2).(int64))
		tx.Release() //-1
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
	log.Info("Starting ZeroMQ Publisher ...")

	notifyNewTx := events.NewClosure(func(transaction *tanglePackage.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSynced() || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) {
				// Not sync
				return
			}
			wasSyncBefore = true
		}

		if (firstSeenLatestMilestoneIndex - latestSolidMilestoneIndex) <= isSyncThreshold {
			transaction.RegisterConsumer() //+1
			_, added := newTxWorkerPool.TrySubmit(transaction)
			if !added {
				transaction.Release() //-1
			}
		}
	})

	notifyConfirmedTx := events.NewClosure(func(transaction *tanglePackage.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if !wasSyncBefore {
			return
		}

		transaction.RegisterConsumer() //+1
		_, added := confirmedTxWorkerPool.TrySubmit(transaction, msIndex, confTime)
		if !added {
			transaction.Release() //-1
		}
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
		log.Info("Starting ZeroMQ Publisher ... done")
		log.Infof("You can now listen to ZMQ via: %s://%s:%d", parameter.NodeConfig.GetString("zmq.protocol"), parameter.NodeConfig.GetString("zmq.host"), parameter.NodeConfig.GetInt("zmq.port"))

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
		log.Info("Stopping ZeroMQ[NewTxWorker] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ...")
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewSolidMilestoneWorker] ...")
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

	return publisher.Start()
}
