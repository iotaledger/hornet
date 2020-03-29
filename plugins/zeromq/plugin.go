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

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglePackage "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
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
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*tanglePackage.CachedTransaction), task.Param(1).(milestone.Index), task.Param(2).(int64)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(confirmedTxWorkerCount), workerpool.QueueSize(confirmedTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewLatestMilestone(task.Param(0).(*tanglePackage.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newLatestMilestoneWorkerCount), workerpool.QueueSize(newLatestMilestoneWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewSolidMilestone(task.Param(0).(*tanglePackage.CachedBundle)) // bundle pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newSolidMilestoneWorkerCount), workerpool.QueueSize(newSolidMilestoneWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	spentAddressWorkerPool = workerpool.New(func(task workerpool.Task) {
		onSpentAddress(task.Param(0).(trinary.Hash))
		task.Return(nil)
	}, workerpool.WorkerCount(spentAddressWorkerCount), workerpool.QueueSize(spentAddressWorkerQueueSize))
}

// Start the zeromq plugin
func run(_ *node.Plugin) {
	log.Info("Starting ZeroMQ Publisher ...")

	notifyNewTx := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, firstSeenLatestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
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

	notifyNewLatestMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if !wasSyncBefore {
			// Not sync
			cachedBndl.Release(true) // tx -1
			return
		}

		if _, added := newLatestMilestoneWorkerPool.TrySubmit(cachedBndl); added { // bundle pass +1
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	notifyNewSolidMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if !wasSyncBefore {
			// Not sync
			cachedBndl.Release(true) // tx -1
			return
		}

		if _, added := newSolidMilestoneWorkerPool.TrySubmit(cachedBndl); added { // bundle pass +1
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	notifySpentAddress := events.NewClosure(func(addr trinary.Hash) {
		spentAddressWorkerPool.TrySubmit(addr)
	})

	daemon.BackgroundWorker("ZeroMQ Publisher", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ Publisher ... done")
		log.Infof("You can now listen to ZMQ via: %s://%s", config.NodeConfig.GetString(config.CfgZMQProtocol), config.NodeConfig.GetString(config.CfgZMQBindAddress))

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
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ address topic updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(updateAddressTopics, 5*time.Second, shutdownSignal)
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		newTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewTxWorker] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ...")
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewSolidMilestoneWorker] ...")
		tangle.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[NewSolidMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(notifySpentAddress)
		spentAddressWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(notifySpentAddress)
		spentAddressWorkerPool.StopAndWait()
		log.Info("Stopping ZeroMQ[SpentAddress] ... done")
	}, shutdown.PriorityMetricsPublishers)
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
