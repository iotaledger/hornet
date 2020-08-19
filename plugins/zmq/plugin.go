package zmq

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

var (
	PLUGIN = node.NewPlugin("ZMQ", node.Disabled, configure, run)
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

// Configure the zmq plugin
func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*tanglePackage.CachedTransaction)) // tx pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize), workerpool.FlushTasksAtShutdown(true))

	confirmedTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onConfirmedTx(task.Param(0).(*tanglePackage.CachedMetadata), task.Param(1).(milestone.Index), task.Param(2).(int64)) // meta pass +1
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

// Start the zmq plugin
func run(_ *node.Plugin) {
	log.Info("Starting ZMQ Publisher ...")

	onReceivedNewTransaction := events.NewClosure(func(cachedTx *tanglePackage.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
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

	onTransactionConfirmed := events.NewClosure(func(cachedMeta *tanglePackage.CachedMetadata, msIndex milestone.Index, confTime int64) {
		if !wasSyncBefore {
			// Not sync
			cachedMeta.Release(true) // meta -1
			return
		}
		// Avoid notifying for conflicting txs
		if !cachedMeta.GetMetadata().IsConflicting() {
			if _, added := confirmedTxWorkerPool.TrySubmit(cachedMeta, msIndex, confTime); added { // meta pass +1
				return // Avoid meta -1 (done inside workerpool task)
			}
		}
		cachedMeta.Release(true) // meta -1
	})

	onLatestMilestoneChanged := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if !wasSyncBefore {
			// Not sync
			cachedBndl.Release(true) // bundle -1
			return
		}

		if _, added := newLatestMilestoneWorkerPool.TrySubmit(cachedBndl); added { // bundle pass +1
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	onSolidMilestoneChanged := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if !wasSyncBefore {
			// Not sync
			cachedBndl.Release(true) // bundle -1
			return
		}

		if _, added := newSolidMilestoneWorkerPool.TrySubmit(cachedBndl); added { // bundle pass +1
			return // Avoid bundle -1 (done inside workerpool task)
		}
		cachedBndl.Release(true) // bundle -1
	})

	onAddressSpent := events.NewClosure(func(addr trinary.Hash) {
		spentAddressWorkerPool.TrySubmit(addr)
	})

	daemon.BackgroundWorker("ZMQ Publisher", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZMQ Publisher ... done")
		log.Infof("You can now listen to ZMQ via: %s://%s", config.NodeConfig.GetString(config.CfgZMQProtocol), config.NodeConfig.GetString(config.CfgZMQBindAddress))

		go func() {
			if err := startPublisher(); err != nil {
				log.Fatal(err)
			}
		}()

		<-shutdownSignal
		log.Info("Stopping ZMQ Publisher ...")

		if err := publisher.Shutdown(); err != nil {
			log.Errorf("Stopping ZMQ Publisher: %s", err.Error())
		} else {
			log.Info("Stopping ZMQ Publisher ... done")
		}
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZMQ address topic updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(updateAddressTopics, 5*time.Second, shutdownSignal)
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZMQ[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZMQ[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(onReceivedNewTransaction)
		newTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZMQ[NewTxWorker] ...")
		tangle.Events.ReceivedNewTransaction.Detach(onReceivedNewTransaction)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping ZMQ[NewTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZMQ[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZMQ[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(onTransactionConfirmed)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZMQ[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(onTransactionConfirmed)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping ZMQ[ConfirmedTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZMQ[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZMQ[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(onLatestMilestoneChanged)
		newLatestMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZMQ[NewLatestMilestoneWorker] ...")
		tangle.Events.LatestMilestoneChanged.Detach(onLatestMilestoneChanged)
		newLatestMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZMQ[NewLatestMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZMQ[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZMQ[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(onSolidMilestoneChanged)
		newSolidMilestoneWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZMQ[NewSolidMilestoneWorker] ...")
		tangle.Events.SolidMilestoneChanged.Detach(onSolidMilestoneChanged)
		newSolidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping ZMQ[NewSolidMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("ZMQ[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZMQ[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(onAddressSpent)
		spentAddressWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping ZMQ[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(onAddressSpent)
		spentAddressWorkerPool.StopAndWait()
		log.Info("Stopping ZMQ[SpentAddress] ... done")
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
