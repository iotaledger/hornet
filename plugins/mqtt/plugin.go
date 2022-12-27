package mqtt

import (
	"context"
	"fmt"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	mqttpkg "github.com/iotaledger/hornet/pkg/mqtt"
	"github.com/iotaledger/hornet/pkg/node"
	"github.com/iotaledger/hornet/pkg/shutdown"
	"github.com/iotaledger/hornet/pkg/tangle"
	"github.com/iotaledger/hornet/plugins/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:      "MQTT",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
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
)

type dependencies struct {
	dig.In
	Storage                               *storage.Storage
	SyncManager                           *syncmanager.SyncManager
	Tangle                                *tangle.Tangle
	NodeConfig                            *configuration.Configuration `name:"nodeConfig"`
	MaxDeltaMsgYoungestConeRootIndexToCMI int                          `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
	MaxDeltaMsgOldestConeRootIndexToCMI   int                          `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
	BelowMaxDepth                         int                          `name:"belowMaxDepth"`
	Bech32HRP                             iotago.NetworkPrefix         `name:"bech32HRP"`
	Echo                                  *echo.Echo                   `optional:"true"`
	MQTTBroker                            *mqttpkg.Broker
}

func provide(c *dig.Container) {

	type brokerDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps brokerDeps) *mqttpkg.Broker {
		mqttBroker, err := mqttpkg.NewBroker(deps.NodeConfig.String(CfgMQTTBindAddress), deps.NodeConfig.Int(CfgMQTTWSPort), "/ws", deps.NodeConfig.Int(CfgMQTTWorkerCount), func(topic []byte) {
			Plugin.LogDebugf("Subscribe to topic: %s", string(topic))
			topicSubscriptionWorkerPool.TrySubmit(topic)
		}, func(topic []byte) {
			Plugin.LogDebugf("Unsubscribe from topic: %s", string(topic))
		}, deps.NodeConfig.Int(CfgMQTTTopicCleanupThreshold))
		if err != nil {
			Plugin.LogFatalf("MQTT broker init failed! %s", err)
		}
		return mqttBroker
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	// check if RestAPI plugin is disabled
	if Plugin.Node.IsSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the MQTT plugin")
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
		publishMessage(task.Param(0).(*storage.CachedMessage)) // message pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	messageMetadataWorkerPool = workerpool.New(func(task workerpool.Task) {
		publishMessageMetadata(task.Param(0).(*storage.CachedMetadata)) // meta pass +1
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
				cachedMsgMeta.Release(true) // meta -1
			}
			return
		}

		if transactionID := transactionIDFromTopic(topicName); transactionID != nil {
			// Find the first output of the transaction
			outputID := &iotago.UTXOInputID{}
			copy(outputID[:], transactionID[:])

			output, err := deps.Storage.UTXOManager().ReadOutputByOutputIDWithoutLocking(outputID)
			if err != nil {
				return
			}

			publishTransactionIncludedMessage(transactionID, output.MessageID())
			return
		}

		if outputID := outputIDFromTopic(topicName); outputID != nil {

			// we need to lock the ledger here to have the correct index for unspent info of the output.
			deps.Storage.UTXOManager().ReadLockLedger()
			defer deps.Storage.UTXOManager().ReadUnlockLedger()

			ledgerIndex, err := deps.Storage.UTXOManager().ReadLedgerIndexWithoutLocking()
			if err != nil {
				return
			}

			output, err := deps.Storage.UTXOManager().ReadOutputByOutputIDWithoutLocking(outputID)
			if err != nil {
				return
			}

			unspent, err := deps.Storage.UTXOManager().IsOutputUnspentWithoutLocking(output)
			if err != nil {
				return
			}
			utxoOutputWorkerPool.TrySubmit(ledgerIndex, output, !unspent)
			return
		}

		if topicName == topicMilestonesLatest {
			index := deps.SyncManager.LatestMilestoneIndex()
			if cachedMilestone := deps.Storage.CachedMilestoneOrNil(index); cachedMilestone != nil { // milestone +1
				publishLatestMilestone(cachedMilestone) // milestone pass +1
			}
			return
		}

		if topicName == topicMilestonesConfirmed {
			index := deps.SyncManager.ConfirmedMilestoneIndex()
			if cachedMilestone := deps.Storage.CachedMilestoneOrNil(index); cachedMilestone != nil { // milestone +1
				publishConfirmedMilestone(cachedMilestone) // milestone pass +1
			}
			return
		}

	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(workerQueueSize), workerpool.FlushTasksAtShutdown(true))

	setupWebSocketRoute()
}

func setupWebSocketRoute() {

	// Configure MQTT WebSocket route
	mqttWSUrl, err := url.Parse(fmt.Sprintf("http://%s:%s", deps.MQTTBroker.Config().Host, deps.MQTTBroker.Config().WsPort))
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
			RouteMQTT: deps.MQTTBroker.Config().WsPath,
		},
	}

	wsGroup.Use(middleware.ProxyWithConfig(proxyConfig))
}

func run() {

	Plugin.LogInfof("Starting MQTT Broker (port %s) ...", deps.MQTTBroker.Config().Port)

	onLatestMilestoneChanged := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		if !wasSyncBefore {
			// Not sync
			cachedMilestone.Release(true) // milestone -1
			return
		}

		if _, added := newLatestMilestoneWorkerPool.TrySubmit(cachedMilestone); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMilestone.Release(true) // milestone -1
	})

	onConfirmedMilestoneChanged := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		if !wasSyncBefore {
			if !deps.SyncManager.IsNodeAlmostSynced() {
				cachedMilestone.Release(true) // milestone -1
				return
			}
			wasSyncBefore = true
		}

		if _, added := newConfirmedMilestoneWorkerPool.TrySubmit(cachedMilestone); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMilestone.Release(true) // milestone -1
	})

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, _ milestone.Index, _ milestone.Index) {
		if !wasSyncBefore {
			// Not sync
			cachedMsg.Release(true) // message -1
			return
		}

		if _, added := messagesWorkerPool.TrySubmit(cachedMsg); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMsg.Release(true) // message -1
	})

	onMessageSolid := events.NewClosure(func(cachedMsgMeta *storage.CachedMetadata) {
		if _, added := messageMetadataWorkerPool.TrySubmit(cachedMsgMeta); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMsgMeta.Release(true) // meta -1
	})

	onMessageReferenced := events.NewClosure(func(cachedMsgMeta *storage.CachedMetadata, _ milestone.Index, _ uint64) {
		if _, added := messageMetadataWorkerPool.TrySubmit(cachedMsgMeta); added {
			return // Avoid Release (done inside workerpool task)
		}
		cachedMsgMeta.Release(true) // meta -1
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

	if err := Plugin.Daemon().BackgroundWorker("MQTT Broker", func(ctx context.Context) {
		go func() {
			deps.MQTTBroker.Start()
			Plugin.LogInfof("Starting MQTT Broker (port %s) ... done", deps.MQTTBroker.Config().Port)
		}()

		if deps.MQTTBroker.Config().Port != "" {
			Plugin.LogInfof("You can now listen to MQTT via: http://%s:%s", deps.MQTTBroker.Config().Host, deps.MQTTBroker.Config().Port)
		}

		if deps.MQTTBroker.Config().TlsPort != "" {
			Plugin.LogInfof("You can now listen to MQTT via: https://%s:%s", deps.MQTTBroker.Config().TlsHost, deps.MQTTBroker.Config().TlsPort)
		}

		<-ctx.Done()
		Plugin.LogInfo("Stopping MQTT Broker ...")
		Plugin.LogInfo("Stopping MQTT Broker ... done")
	}, shutdown.PriorityMetricsPublishers); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := Plugin.Daemon().BackgroundWorker("MQTT Events", func(ctx context.Context) {
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

		<-ctx.Done()

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
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
