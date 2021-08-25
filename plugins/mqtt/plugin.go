package mqtt

import (
	"fmt"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	iotago "github.com/iotaledger/iota.go/v2"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
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
	deps   dependencies

	newLatestMilestoneWorkerPool    *workerpool.WorkerPool
	newConfirmedMilestoneWorkerPool *workerpool.WorkerPool

	messagesWorkerPool        *workerpool.WorkerPool
	messageMetadataWorkerPool *workerpool.WorkerPool
	utxoOutputWorkerPool      *workerpool.WorkerPool
	receiptWorkerPool         *workerpool.WorkerPool

	topicSubscriptionWorkerPool *workerpool.WorkerPool

	wasSyncBefore = false

	mqttBroker *mqttpkg.Broker
)

type dependencies struct {
	dig.In
	Storage                               *storage.Storage
	Tangle                                *tangle.Tangle
	NodeConfig                            *configuration.Configuration `name:"nodeConfig"`
	MaxDeltaMsgYoungestConeRootIndexToCMI int                          `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
	MaxDeltaMsgOldestConeRootIndexToCMI   int                          `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
	BelowMaxDepth                         int                          `name:"belowMaxDepth"`
	Bech32HRP                             iotago.NetworkPrefix         `name:"bech32HRP"`
	Echo                                  *echo.Echo                   `optional:"true"`
}

func configure() {
	// check if RestAPI plugin is disabled
	if Plugin.Node.IsSkipped(restapi.Plugin) {
		Plugin.Panic("RestAPI plugin needs to be enabled to use the MQTT plugin")
	}

	newLatestMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishLatestMilestone(task.Param(0).(*storage.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	newConfirmedMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishConfirmedMilestone(task.Param(0).(*storage.CachedMilestone)) // milestone pass +1
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
		publishOutput(task.Param(0).(milestone.Index), task.Param(1).(*utxo.Output), task.Param(2).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize))

	receiptWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishReceipt(task.Param(0).(*iotago.Receipt))
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize))

	topicSubscriptionWorkerPool = workerpool.New(func(task workerpool.Task) {
		defer task.Return(nil)

		topic := task.Param(0).([]byte)
		topicName := string(topic)

		if messageID := messageIDFromTopic(topicName); messageID != nil {
			if cachedMsgMeta := deps.Storage.CachedMessageMetadataOrNil(messageID); cachedMsgMeta != nil {
				if _, added := messageMetadataWorkerPool.TrySubmit(cachedMsgMeta); added {
					return // Avoid Release (done inside workerpool task)
				}
				cachedMsgMeta.Release(true)
			}
			return
		}

		if transactionID := transactionIDFromTopic(topicName); transactionID != nil {
			// Find the first output of the transaction
			outputID := &iotago.UTXOInputID{}
			copy(outputID[:], transactionID[:])

			output, err := deps.Storage.UTXO().ReadOutputByOutputIDWithoutLocking(outputID)
			if err != nil {
				return
			}

			publishTransactionIncludedMessage(transactionID, output.MessageID())
			return
		}

		if outputID := outputIDFromTopic(topicName); outputID != nil {

			// we need to lock the ledger here to have the correct index for unspent info of the output.
			deps.Storage.UTXO().ReadLockLedger()
			defer deps.Storage.UTXO().ReadUnlockLedger()

			ledgerIndex, err := deps.Storage.UTXO().ReadLedgerIndexWithoutLocking()
			if err != nil {
				return
			}

			output, err := deps.Storage.UTXO().ReadOutputByOutputIDWithoutLocking(outputID)
			if err != nil {
				return
			}

			unspent, err := deps.Storage.UTXO().IsOutputUnspentWithoutLocking(output)
			if err != nil {
				return
			}
			utxoOutputWorkerPool.TrySubmit(ledgerIndex, output, !unspent)
			return
		}

		if topicName == topicMilestonesLatest {
			index := deps.Storage.LatestMilestoneIndex()
			if milestone := deps.Storage.CachedMilestoneOrNil(index); milestone != nil {
				publishLatestMilestone(milestone) // milestone pass +1
			}
			return
		}

		if topicName == topicMilestonesConfirmed {
			index := deps.Storage.ConfirmedMilestoneIndex()
			if milestone := deps.Storage.CachedMilestoneOrNil(index); milestone != nil {
				publishConfirmedMilestone(milestone) // milestone pass +1
			}
			return
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	var err error
	mqttBroker, err = mqttpkg.NewBroker(deps.NodeConfig.String(CfgMQTTBindAddress), deps.NodeConfig.Int(CfgMQTTWSPort), "/ws", deps.NodeConfig.Int(CfgMQTTWorkerCount), func(topic []byte) {
		Plugin.LogInfof("Subscribe to topic: %s", string(topic))
		topicSubscriptionWorkerPool.TrySubmit(topic)
	}, func(topic []byte) {
		Plugin.LogInfof("Unsubscribe from topic: %s", string(topic))
	})

	if err != nil {
		Plugin.LogFatalf("MQTT broker init failed! %s", err)
	}

	setupWebSocketRoute()
}

func setupWebSocketRoute() {

	// Configure MQTT WebSocket route
	mqttWSUrl, err := url.Parse(fmt.Sprintf("http://%s:%s", mqttBroker.Config().Host, mqttBroker.Config().WsPort))
	if err != nil {
		Plugin.LogFatalf("MQTT WebSocket init failed! %s", err)
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
			RouteMQTT: mqttBroker.Config().WsPath,
		},
	}

	wsGroup.Use(middleware.ProxyWithConfig(proxyConfig))
}

func run() {

	Plugin.LogInfof("Starting MQTT Broker (port %s) ...", mqttBroker.Config().Port)

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

	onConfirmedMilestoneChanged := events.NewClosure(func(cachedMs *storage.CachedMilestone) {
		if !wasSyncBefore {
			if !deps.Storage.IsNodeAlmostSynced() {
				cachedMs.Release(true)
				return
			}
			wasSyncBefore = true
		}

		if _, added := newConfirmedMilestoneWorkerPool.TrySubmit(cachedMs); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMs.Release(true)
	})

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, _ milestone.Index, _ milestone.Index) {
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

	onMessageReferenced := events.NewClosure(func(cachedMetadata *storage.CachedMetadata, _ milestone.Index, _ uint64) {
		if _, added := messageMetadataWorkerPool.TrySubmit(cachedMetadata); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMetadata.Release(true)
	})

	onUTXOOutput := events.NewClosure(func(index milestone.Index, output *utxo.Output) {
		utxoOutputWorkerPool.TrySubmit(index, output, false)
	})

	onUTXOSpent := events.NewClosure(func(index milestone.Index, spent *utxo.Spent) {
		utxoOutputWorkerPool.TrySubmit(index, spent.Output(), true)
	})

	onReceipt := events.NewClosure(func(receipt *iotago.Receipt) {
		receiptWorkerPool.TrySubmit(receipt)
	})

	if err := Plugin.Daemon().BackgroundWorker("MQTT Broker", func(shutdownSignal <-chan struct{}) {
		go func() {
			mqttBroker.Start()
			Plugin.LogInfof("Starting MQTT Broker (port %s) ... done", mqttBroker.Config().Port)
		}()

		if mqttBroker.Config().Port != "" {
			Plugin.LogInfof("You can now listen to MQTT via: http://%s:%s", mqttBroker.Config().Host, mqttBroker.Config().Port)
		}

		if mqttBroker.Config().TlsPort != "" {
			Plugin.LogInfof("You can now listen to MQTT via: https://%s:%s", mqttBroker.Config().TlsHost, mqttBroker.Config().TlsPort)
		}

		<-shutdownSignal
		Plugin.LogInfo("Stopping MQTT Broker ...")
		Plugin.LogInfo("Stopping MQTT Broker ... done")
	}, shutdown.PriorityMetricsPublishers); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}

	if err := Plugin.Daemon().BackgroundWorker("MQTT Events", func(shutdownSignal <-chan struct{}) {
		Plugin.LogInfo("Starting MQTT Events ... done")

		deps.Tangle.Events.LatestMilestoneChanged.Attach(onLatestMilestoneChanged)
		deps.Tangle.Events.ConfirmedMilestoneChanged.Attach(onConfirmedMilestoneChanged)

		deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
		deps.Tangle.Events.MessageReferenced.Attach(onMessageReferenced)

		deps.Tangle.Events.NewUTXOOutput.Attach(onUTXOOutput)
		deps.Tangle.Events.NewUTXOSpent.Attach(onUTXOSpent)

		deps.Tangle.Events.NewReceipt.Attach(onReceipt)

		messagesWorkerPool.Start()
		newLatestMilestoneWorkerPool.Start()
		newConfirmedMilestoneWorkerPool.Start()
		messageMetadataWorkerPool.Start()
		topicSubscriptionWorkerPool.Start()
		utxoOutputWorkerPool.Start()
		receiptWorkerPool.Start()

		<-shutdownSignal

		deps.Tangle.Events.LatestMilestoneChanged.Detach(onLatestMilestoneChanged)
		deps.Tangle.Events.ConfirmedMilestoneChanged.Detach(onConfirmedMilestoneChanged)

		deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
		deps.Tangle.Events.MessageReferenced.Detach(onMessageReferenced)

		deps.Tangle.Events.NewUTXOOutput.Detach(onUTXOOutput)
		deps.Tangle.Events.NewUTXOSpent.Detach(onUTXOSpent)

		deps.Tangle.Events.NewReceipt.Detach(onReceipt)

		messagesWorkerPool.StopAndWait()
		newLatestMilestoneWorkerPool.StopAndWait()
		newConfirmedMilestoneWorkerPool.StopAndWait()
		messageMetadataWorkerPool.StopAndWait()
		topicSubscriptionWorkerPool.StopAndWait()
		utxoOutputWorkerPool.StopAndWait()
		receiptWorkerPool.StopAndWait()

		Plugin.LogInfo("Stopping MQTT Events ... done")
	}, shutdown.PriorityMetricsPublishers); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}
