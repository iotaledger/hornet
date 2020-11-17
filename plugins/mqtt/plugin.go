package mqtt

import (
	"fmt"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Disabled,
		Pluggable: node.Pluggable{
			Name:      "MQTT",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

const (
	// RouteMQTT is the route for accessing the MQTT over WebSockets.
	RouteMQTT = "/mqtt"

	workerCount     = 1
	workerQueueSize = 10000
)

var (
	Plugin *node.Plugin
	log    *logger.Logger
	deps   dependencies

	newLatestMilestoneWorkerPool *workerpool.WorkerPool
	newSolidMilestoneWorkerPool  *workerpool.WorkerPool

	messagesWorkerPool        *workerpool.WorkerPool
	messageMetadataWorkerPool *workerpool.WorkerPool
	utxoOutputWorkerPool      *workerpool.WorkerPool

	topicSubscriptionWorkerPool *workerpool.WorkerPool

	wasSyncBefore = false

	mqttBroker *mqttpkg.Broker
)

type dependencies struct {
	dig.In
	Storage    *storage.Storage
	Tangle     *tangle.Tangle
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
	Echo       *echo.Echo
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishLatestMilestone(task.Param(0).(*storage.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishSolidMilestone(task.Param(0).(*storage.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	messagesWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishMessage(task.Param(0).(*storage.CachedMessage)) // metadata pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	messageMetadataWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishMessageMetadata(task.Param(0).(*storage.CachedMetadata)) // metadata pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	utxoOutputWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishOutput(task.Param(0).(*utxo.Output), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	topicSubscriptionWorkerPool = workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		topic := task.Param(0).([]byte)
		topicName := string(topic)

		if messageId := messageIdFromTopic(topicName); messageId != nil {
			if cachedMetadata := deps.Storage.GetCachedMessageMetadataOrNil(messageId); cachedMetadata != nil {
				if _, added := messageMetadataWorkerPool.TrySubmit(cachedMetadata); added {
					return // Avoid Release (done inside workerpool task)
				}
				cachedMetadata.Release(true)
			}
			return
		}

		if outputId := outputIdFromTopic(topicName); outputId != nil {
			output, err := deps.Storage.UTXO().ReadOutputByOutputID(outputId)
			if err != nil {
				return
			}

			unspent, err := deps.Storage.UTXO().IsOutputUnspent(outputId)
			if err != nil {
				return
			}
			utxoOutputWorkerPool.TrySubmit(output, !unspent)
			return
		}

		if topicName == topicMilestonesLatest {
			index := deps.Storage.GetLatestMilestoneIndex()
			if milestone := deps.Storage.GetCachedMilestoneOrNil(index); milestone != nil {
				publishLatestMilestone(milestone) // milestone pass +1
			}
			return
		}

		if topicName == topicMilestonesSolid {
			index := deps.Storage.GetSolidMilestoneIndex()
			if milestone := deps.Storage.GetCachedMilestoneOrNil(index); milestone != nil {
				publishSolidMilestone(milestone) // milestone pass +1
			}
			return
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	var err error
	mqttBroker, err = mqttpkg.NewBroker(deps.NodeConfig.String(CfgMQTTBindAddress), deps.NodeConfig.Int(CfgMQTTWSPort), "/ws", func(topic []byte) {
		log.Infof("Subscribe to topic: %s", string(topic))
		topicSubscriptionWorkerPool.TrySubmit(topic)
	}, func(topic []byte) {
		log.Infof("Unsubscribe from topic: %s", string(topic))
	})

	if err != nil {
		log.Fatalf("MQTT broker init failed! %s", err)
	}

	setupWebSocketRoute()
}

func setupWebSocketRoute() {

	// Configure MQTT WebSocket route
	mqttWSUrl, err := url.Parse(fmt.Sprintf("http://%s:%s", mqttBroker.GetConfig().Host, mqttBroker.GetConfig().WsPort))
	if err != nil {
		log.Fatalf("MQTT WebSocket init failed! %s", err)
	}

	wsGroup := deps.Echo.Group(RouteMQTT)
	proxyConfig := middleware.ProxyConfig{
		Skipper: middleware.DefaultSkipper,
		Balancer: middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
			{
				URL: mqttWSUrl,
			},
		}),
		// We need to forward any calls to the MQTT route to the ws endpoint of our broker
		Rewrite: map[string]string{
			RouteMQTT: mqttBroker.GetConfig().WsPath,
		},
	}

	wsGroup.Use(middleware.ProxyWithConfig(proxyConfig))
}

func run() {

	log.Infof("Starting MQTT Broker (port %s) ...", mqttBroker.GetConfig().Port)

	onLatestMilestoneChanged := events.NewClosure(func(cachedMs *storage.CachedMilestone) {
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

	onSolidMilestoneChanged := events.NewClosure(func(cachedMs *storage.CachedMilestone) {
		if !wasSyncBefore {
			if !deps.Storage.IsNodeSyncedWithThreshold() {
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

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		if !wasSyncBefore {
			// Not sync
			cachedMsg.Release(true)
			return
		}

		if _, added := messagesWorkerPool.TrySubmit(cachedMsg); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMsg.Release(true)
	})

	onMessageSolid := events.NewClosure(func(cachedMetadata *storage.CachedMetadata) {
		if _, added := messageMetadataWorkerPool.TrySubmit(cachedMetadata); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMetadata.Release(true)
	})

	onMessageReferenced := events.NewClosure(func(cachedMetadata *storage.CachedMetadata, msIndex milestone.Index, confTime uint64) {
		if _, added := messageMetadataWorkerPool.TrySubmit(cachedMetadata); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMetadata.Release(true)
	})

	onUTXOOutput := events.NewClosure(func(output *utxo.Output) {
		utxoOutputWorkerPool.TrySubmit(output, false)
	})

	onUTXOSpent := events.NewClosure(func(spent *utxo.Spent) {
		utxoOutputWorkerPool.TrySubmit(spent.Output(), true)
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

		deps.Tangle.Events.LatestMilestoneChanged.Attach(onLatestMilestoneChanged)
		deps.Tangle.Events.SolidMilestoneChanged.Attach(onSolidMilestoneChanged)

		deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
		deps.Tangle.Events.MessageReferenced.Attach(onMessageReferenced)

		deps.Tangle.Events.NewUTXOOutput.Attach(onUTXOOutput)
		deps.Tangle.Events.NewUTXOSpent.Attach(onUTXOSpent)

		messagesWorkerPool.Start()
		newLatestMilestoneWorkerPool.Start()
		newSolidMilestoneWorkerPool.Start()
		messageMetadataWorkerPool.Start()
		topicSubscriptionWorkerPool.Start()
		utxoOutputWorkerPool.Start()

		<-shutdownSignal

		deps.Tangle.Events.LatestMilestoneChanged.Detach(onLatestMilestoneChanged)
		deps.Tangle.Events.SolidMilestoneChanged.Detach(onSolidMilestoneChanged)

		deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
		deps.Tangle.Events.MessageReferenced.Detach(onMessageReferenced)

		deps.Tangle.Events.NewUTXOOutput.Detach(onUTXOOutput)
		deps.Tangle.Events.NewUTXOSpent.Detach(onUTXOSpent)

		messagesWorkerPool.StopAndWait()
		newLatestMilestoneWorkerPool.StopAndWait()
		newSolidMilestoneWorkerPool.StopAndWait()
		messageMetadataWorkerPool.StopAndWait()
		topicSubscriptionWorkerPool.StopAndWait()
		utxoOutputWorkerPool.StopAndWait()

		log.Info("Stopping MQTT Events ... done")
	}, shutdown.PriorityMetricsPublishers)
}
