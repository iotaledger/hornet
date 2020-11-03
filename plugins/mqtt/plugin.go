package mqtt

import (
	"github.com/gohornet/hornet/core/tangle"
	tanglepkg "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/workerpool"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/config"
	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	workerCount     = 1
	workerQueueSize = 10000
)

var (
	Plugin *node.Plugin
	log    *logger.Logger

	newLatestMilestoneWorkerPool *workerpool.WorkerPool
	newSolidMilestoneWorkerPool  *workerpool.WorkerPool

	wasSyncBefore = false

	mqttBroker *mqttpkg.Broker

	deps dependencies
)

type dependencies struct {
	dig.In
	Tangle     *tanglepkg.Tangle
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func init() {
	Plugin = node.NewPlugin("MQTT", node.Disabled, configure, run)
}
func configure(c *dig.Container) {
	log = logger.NewLogger(Plugin.Name)

	if err := c.Invoke(func(cDeps dependencies) {
		deps = cDeps
	}); err != nil {
		panic(err)
	}

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewLatestMilestone(task.Param(0).(*tanglepkg.CachedMilestone))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewSolidMilestone(task.Param(0).(*tanglepkg.CachedMilestone))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	mqttConfigFile := deps.NodeConfig.String(config.CfgMQTTConfig)

	var err error
	mqttBroker, err = mqttpkg.NewBroker(mqttConfigFile)
	if err != nil {
		log.Fatalf("MQTT broker init failed! %v", err.Error())
	}
}

func run(_ *dig.Container) {

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
			if !deps.Tangle.IsNodeSyncedWithThreshold() {
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

	Plugin.Daemon().BackgroundWorker("MQTT Broker", func(shutdownSignal <-chan struct{}) {
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

	Plugin.Daemon().BackgroundWorker("MQTT Events", func(shutdownSignal <-chan struct{}) {
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
