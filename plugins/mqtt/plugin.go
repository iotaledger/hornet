package mqtt

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	logger "github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/packages/model/hornet"
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
	log    = logger.NewLogger("MQTT")

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

	mqttBroker *Broker
)

// Configure the MQTT plugin
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

	var err error
	mqttBroker, err = NewBroker()
	if err != nil {
		log.Fatalf("MQTT broker init failed! %v", err)
	}
}

// Start the MQTT plugin
func run(plugin *node.Plugin) {

	log.Infof("Starting MQTT Broker (port %s) ...", mqttBroker.config.Port)

	notifyNewTx := events.NewClosure(func(transaction *hornet.Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex) {
		if !wasSyncBefore {
			if !tanglePackage.IsNodeSynced() || (firstSeenLatestMilestoneIndex <= tanglePackage.GetLatestSeenMilestoneIndexFromSnapshot()) {
				// Not sync
				return
			}
			wasSyncBefore = true
		}

		if (firstSeenLatestMilestoneIndex - latestSolidMilestoneIndex) <= isSyncThreshold {
			newTxWorkerPool.TrySubmit(transaction)
		}
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
}

// Start the mqtt broker.
func startBroker(plugin *node.Plugin) error {
	return mqttBroker.Start()
}
