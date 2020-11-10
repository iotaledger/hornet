package mqtt

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglepkg "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
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
	Tangle     *tanglepkg.Tangle
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishLatestMilestone(task.Param(0).(*tanglepkg.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newSolidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishSolidMilestone(task.Param(0).(*tanglepkg.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	messagesWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishMessage(task.Param(0).(*tanglepkg.CachedMessage)) // metadata pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	messageMetadataWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishMessageMetadata(task.Param(0).(*tanglepkg.CachedMetadata)) // metadata pass +1
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
			if cachedMetadata := deps.Tangle.GetCachedMessageMetadataOrNil(messageId); cachedMetadata != nil {
				if _, added := messageMetadataWorkerPool.TrySubmit(cachedMetadata); added {
					return // Avoid Release (done inside workerpool task)
				}
				cachedMetadata.Release(true)
			}
			return
		}

		if outputId := outputIdFromTopic(topicName); outputId != nil {
			output, err := deps.Tangle.UTXO().ReadOutputByOutputID(outputId)
			if err != nil {
				return
			}

			unspent, err := deps.Tangle.UTXO().IsOutputUnspent(outputId)
			if err != nil {
				return
			}
			utxoOutputWorkerPool.TrySubmit(output, !unspent)
			return
		}

		if topicName == topicMilestonesLatest {
			index := deps.Tangle.GetLatestMilestoneIndex()
			if milestone := deps.Tangle.GetCachedMilestoneOrNil(index); milestone != nil {
				publishLatestMilestone(milestone) // milestone pass +1
			}
			return
		}

		if topicName == topicMilestonesSolid {
			index := deps.Tangle.GetSolidMilestoneIndex()
			if milestone := deps.Tangle.GetCachedMilestoneOrNil(index); milestone != nil {
				publishSolidMilestone(milestone) // milestone pass +1
			}
			return
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	mqttConfigFile := deps.NodeConfig.String(CfgMQTTConfig)

	var err error
	mqttBroker, err = mqttpkg.NewBroker(mqttConfigFile, func(topic []byte) {
		log.Infof("Subscribe to topic: %s", string(topic))
		topicSubscriptionWorkerPool.TrySubmit(topic)
	}, func(topic []byte) {
		log.Infof("Unsubscribe from topic: %s", string(topic))
	})

	if err != nil {
		log.Fatalf("MQTT broker init failed! %s", err)
	}
}

func run() {

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

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *tanglepkg.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
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

	onMessageSolid := events.NewClosure(func(cachedMetadata *tanglepkg.CachedMetadata) {
		if _, added := messageMetadataWorkerPool.TrySubmit(cachedMetadata); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMetadata.Release(true)
	})

	onMessageReferenced := events.NewClosure(func(cachedMetadata *tanglepkg.CachedMetadata, msIndex milestone.Index, confTime uint64) {
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

		tangle.Events.LatestMilestoneChanged.Attach(onLatestMilestoneChanged)
		tangle.Events.SolidMilestoneChanged.Attach(onSolidMilestoneChanged)

		tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		tangle.Events.MessageSolid.Attach(onMessageSolid)
		tangle.Events.MessageReferenced.Attach(onMessageReferenced)

		tangle.Events.NewUTXOOutput.Attach(onUTXOOutput)
		tangle.Events.NewUTXOSpent.Attach(onUTXOSpent)

		messagesWorkerPool.Start()
		newLatestMilestoneWorkerPool.Start()
		newSolidMilestoneWorkerPool.Start()
		messageMetadataWorkerPool.Start()
		topicSubscriptionWorkerPool.Start()
		utxoOutputWorkerPool.Start()

		<-shutdownSignal

		tangle.Events.LatestMilestoneChanged.Detach(onLatestMilestoneChanged)
		tangle.Events.SolidMilestoneChanged.Detach(onSolidMilestoneChanged)

		tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		tangle.Events.MessageSolid.Detach(onMessageSolid)
		tangle.Events.MessageReferenced.Detach(onMessageReferenced)

		tangle.Events.NewUTXOOutput.Detach(onUTXOOutput)
		tangle.Events.NewUTXOSpent.Detach(onUTXOSpent)

		messagesWorkerPool.StopAndWait()
		newLatestMilestoneWorkerPool.StopAndWait()
		newSolidMilestoneWorkerPool.StopAndWait()
		messageMetadataWorkerPool.StopAndWait()
		topicSubscriptionWorkerPool.StopAndWait()
		utxoOutputWorkerPool.StopAndWait()

		log.Info("Stopping MQTT Events ... done")
	}, shutdown.PriorityMetricsPublishers)
}
