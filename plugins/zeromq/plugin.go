package zeromq

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	tanglePackage "github.com/gohornet/hornet/packages/model/tangle"
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

	newTxWorkerPool              = (&async.NonBlockingWorkerPool{}).Tune(1)
	confirmedTxWorkerPool        = (&async.NonBlockingWorkerPool{}).Tune(1)
	newLatestMilestoneWorkerPool = (&async.NonBlockingWorkerPool{}).Tune(1)
	newSolidMilestoneWorkerPool  = (&async.NonBlockingWorkerPool{}).Tune(1)
	spentAddressWorkerPool       = (&async.NonBlockingWorkerPool{}).Tune(1)

	wasSyncBefore = false

	publisher *Publisher
)

// Configure the zeromq plugin
func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
}

// Start the zeromq plugin
func run(plugin *node.Plugin) {
	log.Info("Starting ZeroMQ Publisher ...")

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

	notifyNewLatestMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if wasSyncBefore {
			if added := newLatestMilestoneWorkerPool.Submit(func() { onNewLatestMilestone(cachedBndl) }); added { // bundle pass +1
				return // Avoid bundle -1 (done inside workerpool task)
			}
		}
		cachedBndl.Release(true) // bundle -1
	})

	notifyNewSolidMilestone := events.NewClosure(func(cachedBndl *tanglePackage.CachedBundle) {
		if wasSyncBefore {
			if added := newSolidMilestoneWorkerPool.Submit(func() { onNewSolidMilestone(cachedBndl) }); added { // bundle pass +1
				return // Avoid bundle -1 (done inside workerpool task)
			}
		}
		cachedBndl.Release(true) // bundle -1
	})

	notifySpentAddress := events.NewClosure(func(addr trinary.Hash) {
		spentAddressWorkerPool.Submit(func() { onSpentAddress(addr) })
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
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ address topic updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(updateAddressTopics, 5*time.Second, shutdownSignal)
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewTxWorker] ...")
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.ShutdownGracefully()
		log.Info("Stopping ZeroMQ[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ...")
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.ShutdownGracefully()
		log.Info("Stopping ZeroMQ[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ...")
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.ShutdownGracefully()
		log.Info("Stopping ZeroMQ[NewLatestMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[NewSolidMilestoneWorker] ...")
		tangle.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.ShutdownGracefully()
		log.Info("Stopping ZeroMQ[NewSolidMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("ZeroMQ[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting ZeroMQ[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(notifySpentAddress)
		<-shutdownSignal
		log.Info("Stopping ZeroMQ[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(notifySpentAddress)
		spentAddressWorkerPool.Shutdown()
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
