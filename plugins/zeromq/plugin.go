package zeromq

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

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

	spentAddressWorkerCount     = 1
	spentAddressWorkerQueueSize = 1000
	spentAddressWorkerPool      *workerpool.WorkerPool

	wasSyncBefore = false

	publisher *Publisher
)

// Configure the zeromq plugin
func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*tanglePackage.CachedTransaction)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*tanglePackage.CachedTransaction), task.Param(1).(milestone_index.MilestoneIndex), task.Param(2).(int64)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize))

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewLatestMilestone(task.Param(0).(*tanglePackage.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newLatestMilestoneWorkerCount), workerpool.QueueSize(newLatestMilestoneWorkerQueueSize))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewSolidMilestone(task.Param(0).(*tanglePackage.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newSolidMilestoneWorkerCount), workerpool.QueueSize(newSolidMilestoneWorkerQueueSize))

	spentAddressWorkerPool = workerpool.New(func(task workerpool.Task) {
		onSpentAddress(task.Param(0).(trinary.Hash))
		task.Return(nil)
	}, workerpool.WorkerCount(spentAddressWorkerCount), workerpool.QueueSize(spentAddressWorkerQueueSize))
}

// Start the zeromq plugin
func run(plugin *node.Plugin) {
	log.Info("Starting ZeroMQ Publisher ...")

	notifyNewTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSynced() || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) {
				// Not sync
				cachedTx.Release() // tx -1
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
		cachedTx.Release() // tx -1
	})

	notifyConfirmedTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
		if wasSyncBefore {
			_, added := confirmedTxWorkerPool.TrySubmit(cachedTx, msIndex, confTime) // tx pass +1
			if added {
				return // Avoid tx -1 (done inside workerpool task)
			}
		}
		cachedTx.Release() // tx -1
	})

	notifyNewLatestMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if wasSyncBefore {
			_, added := newLatestMilestoneWorkerPool.TrySubmit(cachedBndl) // bundle pass +1
			if added {
				return // Avoid bundle -1 (done inside workerpool task)
			}
		}
		cachedBndl.Release() // bundle -1
	})

	notifyNewSolidMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if wasSyncBefore {
			_, added := newSolidMilestoneWorkerPool.TrySubmit(cachedBndl) // bundle pass +1
			if added {
				return // Avoid bundle -1 (done inside workerpool task)
			}
		}
		cachedBndl.Release() // bundle -1
	})

	notifySpentAddress := events.NewClosure(func(addr trinary.Hash) {
		spentAddressWorkerPool.TrySubmit(addr)
	})

	daemon.BackgroundWorker("ZeroMQ Publisher", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ Publisher ... done")
		log.Infof("You can now listen to ZMQ via: %s://%s:%d", parameter.NodeConfig.GetString("zmq.protocol"), parameter.NodeConfig.GetString("zmq.bindAddress"), parameter.NodeConfig.GetInt("zmq.port"))

		go func() {
			if err := startPublisher(); err != nil {
				log.Fatal(err)
			}
		}()

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

	daemon.BackgroundWorker("ZeroMQ[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(notifySpentAddress)
		spentAddressWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(notifySpentAddress)
		spentAddressWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[SpentAddress] ... done")
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
