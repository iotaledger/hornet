package mqtt

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	tanglePackage "github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

const (
	isSyncThreshold = 1
)

var (
	// MQTT is disabled by default
	PLUGIN = node.NewPlugin("MQTT", node.Disabled, configure, run)
	log    *logger.Logger

	newTxWorkerPool              = (&async.NonBlockingWorkerPool{}).Tune(1)
	confirmedTxWorkerPool        = (&async.NonBlockingWorkerPool{}).Tune(1)
	newLatestMilestoneWorkerPool = (&async.NonBlockingWorkerPool{}).Tune(1)
	newSolidMilestoneWorkerPool  = (&async.NonBlockingWorkerPool{}).Tune(1)
	spentAddressWorkerPool       = (&async.NonBlockingWorkerPool{}).Tune(1)

	wasSyncBefore = false

	mqttBroker *Broker
)

// Configure the MQTT plugin
func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	var err error
	mqttBroker, err = NewBroker()
	if err != nil {
		log.Fatalf("MQTT broker init failed! %v", err)
	}
}

// Start the MQTT plugin
func run(plugin *node.Plugin) {

	log.Infof("Starting MQTT Broker (port %s) ...", mqttBroker.config.Port)

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

	daemon.BackgroundWorker("MQTT Broker", func(shutdownSignal <-chan struct{}) {
		go func() {
			if err := startBroker(plugin); err != nil {
				log.Errorf("Stopping MQTT Broker: %s", err.Error())
			} else {
				log.Infof("Starting MQTT Broker (port %s) ... done", mqttBroker.config.Port)
			}

		}()

		if mqttBroker.config.Port != "" {
			log.Infof("You can now listen to MQTT via: http://%s:%s", mqttBroker.config.Host, mqttBroker.config.Port)
		}

		if mqttBroker.config.TlsPort != "" {
			log.Infof("You can now listen to MQTT via: https://%s:%s", mqttBroker.config.TlsHost, mqttBroker.config.TlsPort)
		}

		<-shutdownSignal
		log.Info("Stopping MQTT Broker ...")

		if err := mqttBroker.Shutdown(); err != nil {
			log.Errorf("Stopping MQTT Broker: %s", err.Error())
		} else {
			log.Info("Stopping MQTT Broker ... done")
		}
	}, shutdown.ShutdownPriorityMetricsPublishers)

	/*
		daemon.BackgroundWorker("MQTT address topic updater", func(shutdownSignal <-chan struct{}) {
			timeutil.Ticker(updateAddressTopics, 5*time.Second, shutdownSignal)
		}, shutdown.ShutdownPriorityMetricsPublishers)
	*/

	daemon.BackgroundWorker("MQTT[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		<-shutdownSignal
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.ShutdownGracefully()
		log.Info("Stopping MQTT[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		<-shutdownSignal
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.ShutdownGracefully()
		log.Info("Stopping MQTT[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		<-shutdownSignal
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.ShutdownGracefully()
		log.Info("Stopping MQTT[NewLatestMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		<-shutdownSignal
		tangle.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.ShutdownGracefully()
		log.Info("Stopping MQTT[NewSolidMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(notifySpentAddress)
		<-shutdownSignal
		log.Info("Stopping MQTT[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(notifySpentAddress)
		spentAddressWorkerPool.Shutdown()
		log.Info("Stopping MQTT[SpentAddress] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}

// Start the mqtt broker.
func startBroker(plugin *node.Plugin) error {
	return mqttBroker.Start()
}
