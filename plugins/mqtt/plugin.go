package mqtt

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/config"
	tanglepkg "github.com/gohornet/hornet/pkg/model/tangle"
	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	workerCount     = 1
	workerQueueSize = 10000
)

var (
	PLUGIN = node.NewPlugin("MQTT", node.Disabled, configure, run)
	log    *logger.Logger

	newLatestMilestoneWorkerPool *workerpool.WorkerPool
	newSolidMilestoneWorkerPool  *workerpool.WorkerPool

	wasSyncBefore = false

	mqttBroker *mqttpkg.Broker
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewLatestMilestone(task.Param(0).(*tanglepkg.CachedMilestone))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewSolidMilestone(task.Param(0).(*tanglepkg.CachedMilestone))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	mqttConfigFile := config.NodeConfig.String(config.CfgMQTTConfig)

	var err error
	mqttBroker, err = mqttpkg.NewBroker(mqttConfigFile)
	if err != nil {
		log.Fatalf("MQTT broker init failed! %v", err.Error())
	}
}

func run(plugin *node.Plugin) {

	log.Infof("Starting MQTT Broker (port %s) ...", mqttBroker.GetConfig().Port)

	onLatestMilestoneChanged := events.NewClosure(func(cachedMs *tanglepkg.CachedMilestone) {
		if !wasSyncBefore {
			// Not sync
			cachedMs.Release(true)
			return
		}

		if _, added := newLatestMilestoneWorkerPool.TrySubmit(cachedMs); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMs.Release(true)
	})

	onSolidMilestoneChanged := events.NewClosure(func(cachedMs *tanglepkg.CachedMilestone) {
		if !wasSyncBefore {
			if !database.Tangle().IsNodeSyncedWithThreshold() {
				cachedMs.Release(true)
				return
			}
			wasSyncBefore = true
		}

		if _, added := newSolidMilestoneWorkerPool.TrySubmit(cachedMs); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMs.Release(true)
	})

	daemon.BackgroundWorker("MQTT Broker", func(shutdownSignal <-chan struct{}) {
		go func() {
			mqttBroker.Start()
			log.Infof("Starting MQTT Broker (port %s) ... done", mqttBroker.GetConfig().Port)
		}()

		if mqttBroker.GetConfig().Port != "" {
			log.Infof("You can now listen to MQTT via: http://%s:%s", mqttBroker.GetConfig().Host, mqttBroker.GetConfig().Port)
		}

		if mqttBroker.GetConfig().TlsPort != "" {
			log.Infof("You can now listen to MQTT via: https://%s:%s", mqttBroker.GetConfig().TlsHost, mqttBroker.GetConfig().TlsPort)
		}

		<-shutdownSignal
		log.Info("Stopping MQTT Broker ...")
		log.Info("Stopping MQTT Broker ... done")
	}, shutdown.PriorityMetricsPublishers)

	daemon.BackgroundWorker("MQTT Events", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting MQTT Events ... done")

		tangle.Events.LatestMilestoneChanged.Attach(onLatestMilestoneChanged)
		tangle.Events.SolidMilestoneChanged.Attach(onSolidMilestoneChanged)

		newLatestMilestoneWorkerPool.Start()
		newSolidMilestoneWorkerPool.Start()

		<-shutdownSignal

		tangle.Events.LatestMilestoneChanged.Detach(onLatestMilestoneChanged)
		tangle.Events.SolidMilestoneChanged.Detach(onSolidMilestoneChanged)

		newLatestMilestoneWorkerPool.StopAndWait()
		newSolidMilestoneWorkerPool.StopAndWait()

		log.Info("Stopping MQTT Events ... done")
	}, shutdown.PriorityMetricsPublishers)
}
