package tangle

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
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

func configureTangleProcessor() {

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

func runTangleProcessor() {
	log.Info("Starting TangleProcessor ...")

	startWaitGroup.Add(4)

	onMsgProcessed := events.NewClosure(func(message *tangle.Message, request *gossippkg.Request, proto *gossippkg.Protocol) {
		receiveMsgWorkerPool.Submit(message, request, proto)
	})

	onMPSMetricsUpdated := events.NewClosure(func(mpsMetrics *MPSMetrics) {
		lastIncomingMPS = mpsMetrics.Incoming
		lastNewMPS = mpsMetrics.New
		lastOutgoingMPS = mpsMetrics.Outgoing
	})

	onReceivedValidMilestone := events.NewClosure(func(cachedMilestone *tangle.CachedMilestone) {
		_, added := processValidMilestoneWorkerPool.Submit(cachedMilestone) // milestone pass +1
		if !added {
			// Release shouldn't be forced, to cache the latest milestones
			cachedMilestone.Release() // message -1
		}
	})

	CorePlugin.Daemon().BackgroundWorker("TangleProcessor[UpdateMetrics]", func(shutdownSignal <-chan struct{}) {
		Events.MPSMetricsUpdated.Attach(onMPSMetricsUpdated)
		startWaitGroup.Done()
		<-shutdownSignal
		Events.MPSMetricsUpdated.Detach(onMPSMetricsUpdated)
	}, shutdown.PriorityMetricsUpdater)

	CorePlugin.Daemon().BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		deps.MessageProcessor.Events.MessageProcessed.Attach(onMsgProcessed)
		receiveMsgWorkerPool.Start()
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		deps.MessageProcessor.Events.MessageProcessed.Detach(onMsgProcessed)
		receiveMsgWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ReceiveTx] ... done")
	}, shutdown.PriorityReceiveTxWorker)

	CorePlugin.Daemon().BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		processValidMilestoneWorkerPool.Start()
		deps.Tangle.Events.ReceivedValidMilestone.Attach(onReceivedValidMilestone)
		startWaitGroup.Done()
		<-shutdownSignal
		log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		deps.Tangle.Events.ReceivedValidMilestone.Detach(onReceivedValidMilestone)
		processValidMilestoneWorkerPool.StopAndWait()
		log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.PriorityMilestoneProcessor)

	CorePlugin.Daemon().BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
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

	latestMilestoneIndex := deps.Tangle.GetLatestMilestoneIndex()
	isNodeSyncedWithThreshold := deps.Tangle.IsNodeSyncedWithThreshold()

	// The msg will be added to the storage inside this function, so the message object automatically updates
	cachedMsg, alreadyAdded := deps.Tangle.AddMessageToStorage(incomingMsg, latestMilestoneIndex, request != nil, !isNodeSyncedWithThreshold, false) // msg +1

	// Release shouldn't be forced, to cache the latest messages
	defer cachedMsg.Release(!isNodeSyncedWithThreshold) // msg -1

	if !alreadyAdded {
		deps.ServerMetrics.NewMessages.Inc()

		if proto != nil {
			proto.Metrics.NewMessages.Inc()
		}

		// since we only add the parents if there was a source request, we only
		// request them for messages which should be part of milestone cones
		if request != nil {
			// add this newly received message's parents to the request queue
			gossip.RequestParents(cachedMsg.Retain(), request.MilestoneIndex, true)
		}

		solidMilestoneIndex := deps.Tangle.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		Events.ReceivedNewMessage.Trigger(cachedMsg, latestMilestoneIndex, solidMilestoneIndex)

	} else {
		deps.ServerMetrics.KnownMessages.Inc()
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
		deps.RequestQueue.Processed(incomingMsg.GetMessageID())
	}

	// we check whether the request is nil, so we only trigger the solidifier when
	// we actually handled a message stemming from a request (as otherwise the solidifier
	// is triggered too often through messages received from normal gossip)
	if !deps.Tangle.IsNodeSynced() && request != nil && deps.RequestQueue.Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
	}
}

// RegisterMessageProcessedEvent returns a channel that gets closed when the message is processed.
func RegisterMessageProcessedEvent(messageID *hornet.MessageID) chan struct{} {
	return messageProcessedSyncEvent.RegisterEvent(messageID.MapKey())
}

// DeregisterMessageProcessedEvent removes a registed event to free the memory if not used.
func DeregisterMessageProcessedEvent(messageID *hornet.MessageID) {
	messageProcessedSyncEvent.DeregisterEvent(messageID.MapKey())
}

// RegisterMessageSolidEvent returns a channel that gets closed when the message is marked as solid.
func RegisterMessageSolidEvent(messageID *hornet.MessageID) chan struct{} {
	return messageSolidSyncEvent.RegisterEvent(messageID.MapKey())
}

// DeregisterMessageSolidEvent removes a registed event to free the memory if not used.
func DeregisterMessageSolidEvent(messageID *hornet.MessageID) {
	messageSolidSyncEvent.DeregisterEvent(messageID.MapKey())
}

// RegisterMilestoneConfirmedEvent returns a channel that gets closed when the milestone is confirmed.
func RegisterMilestoneConfirmedEvent(msIndex milestone.Index) chan struct{} {
	return milestoneConfirmedSyncEvent.RegisterEvent(msIndex)
}

// DeregisterMilestoneConfirmedEvent removes a registed event to free the memory if not used.
func DeregisterMilestoneConfirmedEvent(msIndex milestone.Index) {
	milestoneConfirmedSyncEvent.DeregisterEvent(msIndex)
}

func printStatus() {
	var currentLowestMilestoneIndexInReqQ milestone.Index
	if peekedRequest := deps.RequestQueue.Peek(); peekedRequest != nil {
		currentLowestMilestoneIndexInReqQ = peekedRequest.MilestoneIndex
	}

	queued, pending, processing := deps.RequestQueue.Size()
	avgLatency := deps.RequestQueue.AvgLatency()

	println(
		fmt.Sprintf(
			"req(qu/pe/proc/lat): %05d/%05d/%05d/%04dms, "+
				"reqQMs: %d, "+
				"processor: %05d, "+
				"LSMI/LMI: %d/%d, "+
				"MPS (in/new/out): %05d/%05d/%05d, "+
				"Tips (non-/semi-lazy): %d/%d",
			queued, pending, processing, avgLatency,
			currentLowestMilestoneIndexInReqQ,
			receiveMsgWorkerPool.GetPendingQueueSize(),
			deps.Tangle.GetSolidMilestoneIndex(),
			deps.Tangle.GetLatestMilestoneIndex(),
			lastIncomingMPS,
			lastNewMPS,
			lastOutgoingMPS,
			deps.ServerMetrics.TipsNonLazy.Load(),
			deps.ServerMetrics.TipsSemiLazy.Load()))
}
