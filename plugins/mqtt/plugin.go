package mqtt

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglePackage "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

var (
	// MQTT is disabled by default
	PLUGIN = node.NewPlugin("MQTT", node.Disabled, configure, run)
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

	mqttBroker *Broker
)

// Configure the MQTT plugin
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

	var err error
	mqttBroker, err = NewBroker()
	if err != nil {
		log.Fatalf("MQTT broker init failed! %v", err)
	}
}

// Start the MQTT plugin
func run(plugin *node.Plugin) {

	log.Infof("Starting MQTT Broker (port %s) ...", mqttBroker.config.Port)

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
	}, shutdown.PriorityMetricsPublishers)

	/*
		daemon.BackgroundWorker("MQTT address topic updater", func(shutdownSignal <-chan struct{}) {
			timeutil.Ticker(updateAddressTopics, 5*time.Second, shutdownSignal)
		}, shutdown.PriorityMetricsPublishers)
	*/

	daemon.BackgroundWorker("MQTT[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewTxWorker] ... done")
		tangle.Events.ReceivedNewTransaction.Attach(notifyNewTx)
		newTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[NewTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[ConfirmedTxWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[NewLatestMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[NewSolidMilestoneWorker] ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(notifySpentAddress)
		spentAddressWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping MQTT[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(notifySpentAddress)
		spentAddressWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[SpentAddress] ... done")
	}, shutdown.PriorityMetricsPublishers)
}

// Start the mqtt broker.
func startBroker(_ *node.Plugin) error {
	return mqttBroker.Start()
}
