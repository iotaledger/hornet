package tangle

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/gossip"
	metricsplugin "github.com/gohornet/hornet/plugins/metrics"
)

var (
	receiveMsgWorkerCount = 2 * runtime.NumCPU()
	receiveMsgQueueSize   = 10000
	receiveMsgWorkerPool  *workerpool.WorkerPool

	lastIncomingMPS uint32
	lastNewMPS      uint32
	lastOutgoingMPS uint32

	startWaitGroup sync.WaitGroup

	messageProcessedSyncEvent   = utils.NewSyncEvent()
	messageSolidSyncEvent       = utils.NewSyncEvent()
	milestoneConfirmedSyncEvent = utils.NewSyncEvent()
)

func configureTangleProcessor(_ *node.Plugin) {

	receiveMsgWorkerPool = workerpool.New(func(task workerpool.Task) {
		processIncomingTx(task.Param(0).(*tangle.Message), task.Param(1).(*gossippkg.Request), task.Param(2).(*gossippkg.Protocol))
		task.Return(nil)
	}, workerpool.WorkerCount(receiveMsgWorkerCount), workerpool.QueueSize(receiveMsgQueueSize))

	processValidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		processValidMilestone(task.Param(0).(*tangle.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(processValidMilestoneWorkerCount), workerpool.QueueSize(processValidMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		solidifyMilestone(task.Param(0).(milestone.Index), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(milestoneSolidifierWorkerCount), workerpool.QueueSize(milestoneSolidifierQueueSize))
}

func runTangleProcessor(_ *node.Plugin) {
	log.Info("Starting TangleProcessor ...")

	startWaitGroup.Add(4)

	onMsgProcessed := events.NewClosure(func(message *tangle.Message, request *gossippkg.Request, proto *gossippkg.Protocol) {
		receiveMsgWorkerPool.Submit(message, request, proto)
	})

	onTPSMetricsUpdated := events.NewClosure(func(tpsMetrics *metricsplugin.TPSMetrics) {
		lastIncomingMPS = tpsMetrics.Incoming
		lastNewMPS = tpsMetrics.New
		lastOutgoingMPS = tpsMetrics.Outgoing
	})

	onReceivedValidMilestone := events.NewClosure(func(cachedMilestone *tangle.CachedMilestone) {
		_, added := processValidMilestoneWorkerPool.Submit(cachedMilestone) // milestone pass +1
		if !added {
			// Release shouldn't be forced, to cache the latest milestones
			cachedMilestone.Release() // message -1
		}
	})

	onReceivedInvalidMilestone := events.NewClosure(func(err error) {
		log.Info(err)
	})

	daemon.BackgroundWorker("TangleProcessor[UpdateMetrics]", func(shutdownSignal <-chan struct{}) {
		metricsplugin.Events.TPSMetricsUpdated.Attach(onTPSMetricsUpdated)
		startWaitGroup.Done()
		<-shutdownSignal
		metricsplugin.Events.TPSMetricsUpdated.Detach(onTPSMetricsUpdated)
	}, shutdown.PriorityMetricsUpdater)

	daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		gossip.MessageProcessor().Events.MessageProcessed.Attach(onMsgProcessed)
		receiveMsgWorkerPool.Start()
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		gossip.MessageProcessor().Events.MessageProcessed.Detach(onMsgProcessed)
		receiveMsgWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ReceiveTx] ... done")
	}, shutdown.PriorityReceiveTxWorker)

	daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		processValidMilestoneWorkerPool.Start()
		tangle.Events.ReceivedValidMilestone.Attach(onReceivedValidMilestone)
		tangle.Events.ReceivedInvalidMilestone.Attach(onReceivedInvalidMilestone)
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		tangle.Events.ReceivedValidMilestone.Detach(onReceivedValidMilestone)
		tangle.Events.ReceivedInvalidMilestone.Detach(onReceivedInvalidMilestone)
		processValidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.PriorityMilestoneProcessor)

	daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[MilestoneSolidifier] ... done")
		milestoneSolidifierWorkerPool.Start()
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ...")
		milestoneSolidifierWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, shutdown.PriorityMilestoneSolidifier)

}

// WaitForTangleProcessorStartup waits until all background workers of the tangle processor are started.
func WaitForTangleProcessorStartup() {
	startWaitGroup.Wait()
}

func IsReceiveTxWorkerPoolBusy() bool {
	return receiveMsgWorkerPool.GetPendingQueueSize() > (receiveMsgQueueSize / 2)
}

func processIncomingTx(incomingMsg *tangle.Message, request *gossippkg.Request, proto *gossippkg.Protocol) {

	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	isNodeSyncedWithThreshold := tangle.IsNodeSyncedWithThreshold()

	// The msg will be added to the storage inside this function, so the message object automatically updates
	cachedMsg, alreadyAdded := tangle.AddMessageToStorage(incomingMsg, latestMilestoneIndex, request != nil, !isNodeSyncedWithThreshold, false) // msg +1

	// Release shouldn't be forced, to cache the latest messages
	defer cachedMsg.Release(!isNodeSyncedWithThreshold) // msg -1

	if !alreadyAdded {
		metrics.SharedServerMetrics.NewMessages.Inc()

		if proto != nil {
			proto.Metrics.NewMessages.Inc()
		}

		// since we only add the parents if there was a source request, we only
		// request them for messages which should be part of milestone cones
		if request != nil {
			// add this newly received message's parents to the request queue
			gossip.RequestParents(cachedMsg.Retain(), request.MilestoneIndex, true)
		}

		solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		Events.ReceivedNewMessage.Trigger(cachedMsg, latestMilestoneIndex, solidMilestoneIndex)

	} else {
		metrics.SharedServerMetrics.KnownMessages.Inc()
		if proto != nil {
			proto.Metrics.KnownMessages.Inc()
		}
		Events.ReceivedKnownMessage.Trigger(cachedMsg)
	}

	// "ProcessedMessage" event has to be fired after "ReceivedNewMessage" event,
	// otherwise there is a race condition in the coordinator plugin that tries to "ComputeMerkleTreeRootHash"
	// with the message it issued itself because the message may be not solid yet and therefore their database entries
	// are not created yet.
	Events.ProcessedMessage.Trigger(incomingMsg.GetMessageID())
	messageProcessedSyncEvent.Trigger(incomingMsg.GetMessageID().MapKey())

	if request != nil {
		// mark the received request as processed
		gossip.RequestQueue().Processed(incomingMsg.GetMessageID())
	}

	// we check whether the request is nil, so we only trigger the solidifier when
	// we actually handled a message stemming from a request (as otherwise the solidifier
	// is triggered too often through messages received from normal gossip)
	if !tangle.IsNodeSynced() && request != nil && gossip.RequestQueue().Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
	}
}

// RegisterMessageProcessedEvent returns a channel that gets closed when the message is processed.
func RegisterMessageProcessedEvent(messageID *hornet.MessageID) chan struct{} {
	return messageProcessedSyncEvent.RegisterEvent(messageID.MapKey())
}

// RegisterMessageSolidEvent returns a channel that gets closed when the message is marked as solid.
func RegisterMessageSolidEvent(messageID *hornet.MessageID) chan struct{} {
	return messageSolidSyncEvent.RegisterEvent(messageID.MapKey())
}

// RegisterMilestoneConfirmedEvent returns a channel that gets closed when the milestone is confirmed.
func RegisterMilestoneConfirmedEvent(msIndex milestone.Index) chan struct{} {
	return milestoneConfirmedSyncEvent.RegisterEvent(msIndex)
}

func printStatus() {
	var currentLowestMilestoneIndexInReqQ milestone.Index
	if peekedRequest := gossip.RequestQueue().Peek(); peekedRequest != nil {
		currentLowestMilestoneIndexInReqQ = peekedRequest.MilestoneIndex
	}

	queued, pending, processing := gossip.RequestQueue().Size()
	avgLatency := gossip.RequestQueue().AvgLatency()

	println(
		fmt.Sprintf(
			"req(qu/pe/proc/lat): %05d/%05d/%05d/%04dms, "+
				"reqQMs: %d, "+
				"processor: %05d, "+
				"LSMI/LMI: %d/%d, "+
				"TPS (in/new/out): %05d/%05d/%05d, "+
				"Tips (non-/semi-lazy): %d/%d",
			queued, pending, processing, avgLatency,
			currentLowestMilestoneIndexInReqQ,
			receiveMsgWorkerPool.GetPendingQueueSize(),
			tangle.GetSolidMilestoneIndex(),
			tangle.GetLatestMilestoneIndex(),
			lastIncomingMPS,
			lastNewMPS,
			lastOutgoingMPS,
			metrics.SharedServerMetrics.TipsNonLazy.Load(),
			metrics.SharedServerMetrics.TipsSemiLazy.Load()))
}
