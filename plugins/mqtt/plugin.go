package mqtt

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

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
		newTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.ReceivedNewTransaction.Detach(notifyNewTx)
		newTxWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[ConfirmedTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[ConfirmedTxWorker] ... done")
		tangle.Events.TransactionConfirmed.Attach(notifyConfirmedTx)
		confirmedTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.TransactionConfirmed.Detach(notifyConfirmedTx)
		confirmedTxWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[ConfirmedTxWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[NewLatestMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewLatestMilestoneWorker] ... done")
		tangle.Events.LatestMilestoneChanged.Attach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.LatestMilestoneChanged.Detach(notifyNewLatestMilestone)
		newLatestMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[NewLatestMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[NewSolidMilestoneWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[NewSolidMilestoneWorker] ... done")
		tangle.Events.SolidMilestoneChanged.Attach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.SolidMilestoneChanged.Detach(notifyNewSolidMilestone)
		newSolidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[NewSolidMilestoneWorker] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT[SpentAddress]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT[SpentAddress] ... done")
		tanglePackage.Events.AddressSpent.Attach(notifySpentAddress)
		spentAddressWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping MQTT[SpentAddress] ...")
		tanglePackage.Events.AddressSpent.Detach(notifySpentAddress)
		spentAddressWorkerPool.StopAndWait()
		log.Info("Stopping MQTT[SpentAddress] ... done")
	}, shutdown.ShutdownPriorityMetricsPublishers)
}

// Start the mqtt broker.
func startBroker(plugin *node.Plugin) error {
	return mqttBroker.Start()
}
