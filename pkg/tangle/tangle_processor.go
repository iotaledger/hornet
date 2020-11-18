package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
)

func (t *Tangle) ConfigureTangleProcessor() {

	t.receiveMsgWorkerPool = workerpool.New(func(task workerpool.Task) {
		t.processIncomingTx(task.Param(0).(*storage.Message), task.Param(1).(*gossippkg.Request), task.Param(2).(*gossippkg.Protocol))
		task.Return(nil)
	}, workerpool.WorkerCount(t.receiveMsgWorkerCount), workerpool.QueueSize(t.receiveMsgQueueSize))

	processValidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		t.processValidMilestone(task.Param(0).(*storage.CachedMilestone)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(processValidMilestoneWorkerCount), workerpool.QueueSize(processValidMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	t.milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		t.solidifyMilestone(task.Param(0).(milestone.Index), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(milestoneSolidifierWorkerCount), workerpool.QueueSize(milestoneSolidifierQueueSize))
}

func (t *Tangle) RunTangleProcessor() {
	t.log.Info("Starting TangleProcessor ...")

	// set latest known milestone from database
	latestMilestoneFromDatabase := t.storage.SearchLatestMilestoneIndexInStore()
	if latestMilestoneFromDatabase < t.storage.GetSolidMilestoneIndex() {
		latestMilestoneFromDatabase = t.storage.GetSolidMilestoneIndex()
	}

	t.storage.SetLatestMilestoneIndex(latestMilestoneFromDatabase, t.updateSyncedAtStartup)

	t.startWaitGroup.Add(4)

	onMsgProcessed := events.NewClosure(func(message *storage.Message, request *gossippkg.Request, proto *gossippkg.Protocol) {
		t.receiveMsgWorkerPool.Submit(message, request, proto)
	})

	onMPSMetricsUpdated := events.NewClosure(func(mpsMetrics *MPSMetrics) {
		t.lastIncomingMPS = mpsMetrics.Incoming
		t.lastNewMPS = mpsMetrics.New
		t.lastOutgoingMPS = mpsMetrics.Outgoing
	})

	onReceivedValidMilestone := events.NewClosure(func(cachedMilestone *storage.CachedMilestone) {
		_, added := processValidMilestoneWorkerPool.Submit(cachedMilestone) // milestone pass +1
		if !added {
			// Release shouldn't be forced, to cache the latest milestones
			cachedMilestone.Release() // message -1
		}
	})

	// create a background worker that "measures" the MPS value every second
	t.daemon.BackgroundWorker("Metrics MPS Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(t.measureMPS, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityMetricsUpdater)

	t.daemon.BackgroundWorker("TangleProcessor[UpdateMetrics]", func(shutdownSignal <-chan struct{}) {
		t.Events.MPSMetricsUpdated.Attach(onMPSMetricsUpdated)
		t.startWaitGroup.Done()
		<-shutdownSignal
		t.Events.MPSMetricsUpdated.Detach(onMPSMetricsUpdated)
	}, shutdown.PriorityMetricsUpdater)

	t.daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(shutdownSignal <-chan struct{}) {
		t.log.Info("Starting TangleProcessor[ReceiveTx] ... done")
		t.messageProcessor.Events.MessageProcessed.Attach(onMsgProcessed)
		t.receiveMsgWorkerPool.Start()
		t.startWaitGroup.Done()
		<-shutdownSignal
		t.log.Info("Stopping TangleProcessor[ReceiveTx] ...")
		t.messageProcessor.Events.MessageProcessed.Detach(onMsgProcessed)
		t.receiveMsgWorkerPool.StopAndWait()
		t.log.Info("Stopping TangleProcessor[ReceiveTx] ... done")
	}, shutdown.PriorityReceiveTxWorker)

	t.daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(shutdownSignal <-chan struct{}) {
		t.log.Info("Starting TangleProcessor[ProcessMilestone] ... done")
		processValidMilestoneWorkerPool.Start()
		t.storage.Events.ReceivedValidMilestone.Attach(onReceivedValidMilestone)
		t.startWaitGroup.Done()
		<-shutdownSignal
		t.log.Info("Stopping TangleProcessor[ProcessMilestone] ...")
		t.storage.Events.ReceivedValidMilestone.Detach(onReceivedValidMilestone)
		processValidMilestoneWorkerPool.StopAndWait()
		t.log.Info("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, shutdown.PriorityMilestoneProcessor)

	t.daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(shutdownSignal <-chan struct{}) {
		t.log.Info("Starting TangleProcessor[MilestoneSolidifier] ... done")
		t.milestoneSolidifierWorkerPool.Start()
		t.startWaitGroup.Done()
		<-shutdownSignal
		t.log.Info("Stopping TangleProcessor[MilestoneSolidifier] ...")
		t.milestoneSolidifierWorkerPool.StopAndWait()
		t.log.Info("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, shutdown.PriorityMilestoneSolidifier)

}

// WaitForTangleProcessorStartup waits until all background workers of the tangle processor are started.
func (t *Tangle) WaitForTangleProcessorStartup() {
	t.startWaitGroup.Wait()
}

func (t *Tangle) IsReceiveTxWorkerPoolBusy() bool {
	return t.receiveMsgWorkerPool.GetPendingQueueSize() > (t.receiveMsgQueueSize / 2)
}

func (t *Tangle) processIncomingTx(incomingMsg *storage.Message, request *gossippkg.Request, proto *gossippkg.Protocol) {

	latestMilestoneIndex := t.storage.GetLatestMilestoneIndex()
	isNodeSyncedWithThreshold := t.storage.IsNodeSyncedWithThreshold()

	// The msg will be added to the storage inside this function, so the message object automatically updates
	cachedMsg, alreadyAdded := t.storage.AddMessageToStorage(incomingMsg, latestMilestoneIndex, request != nil, !isNodeSyncedWithThreshold, false) // msg +1

	// Release shouldn't be forced, to cache the latest messages
	defer cachedMsg.Release(!isNodeSyncedWithThreshold) // msg -1

	if !alreadyAdded {
		t.serverMetrics.NewMessages.Inc()

		if proto != nil {
			proto.Metrics.NewMessages.Inc()
		}

		// since we only add the parents if there was a source request, we only
		// request them for messages which should be part of milestone cones
		if request != nil {
			// add this newly received message's parents to the request queue
			t.requester.RequestParents(cachedMsg.Retain(), request.MilestoneIndex, true)
		}

		solidMilestoneIndex := t.storage.GetSolidMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = solidMilestoneIndex
		}
		t.Events.ReceivedNewMessage.Trigger(cachedMsg, latestMilestoneIndex, solidMilestoneIndex)

	} else {
		t.serverMetrics.KnownMessages.Inc()
		if proto != nil {
			proto.Metrics.KnownMessages.Inc()
		}
		t.Events.ReceivedKnownMessage.Trigger(cachedMsg)
	}

	// "ProcessedMessage" event has to be fired after "ReceivedNewMessage" event,
	// otherwise there is a race condition in the coordinator plugin that tries to "ComputeMerkleTreeRootHash"
	// with the message it issued itself because the message may be not solid yet and therefore their database entries
	// are not created yet.
	t.Events.ProcessedMessage.Trigger(incomingMsg.GetMessageID())
	t.messageProcessedSyncEvent.Trigger(incomingMsg.GetMessageID().MapKey())

	if request != nil {
		// mark the received request as processed
		t.requestQueue.Processed(incomingMsg.GetMessageID())
	}

	// we check whether the request is nil, so we only trigger the solidifier when
	// we actually handled a message stemming from a request (as otherwise the solidifier
	// is triggered too often through messages received from normal gossip)
	if !t.storage.IsNodeSynced() && request != nil && t.requestQueue.Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
	}
}

// RegisterMessageProcessedEvent returns a channel that gets closed when the message is processed.
func (t *Tangle) RegisterMessageProcessedEvent(messageID *hornet.MessageID) chan struct{} {
	return t.messageProcessedSyncEvent.RegisterEvent(messageID.MapKey())
}

// DeregisterMessageProcessedEvent removes a registed event to free the memory if not used.
func (t *Tangle) DeregisterMessageProcessedEvent(messageID *hornet.MessageID) {
	t.messageProcessedSyncEvent.DeregisterEvent(messageID.MapKey())
}

// RegisterMessageSolidEvent returns a channel that gets closed when the message is marked as solid.
func (t *Tangle) RegisterMessageSolidEvent(messageID *hornet.MessageID) chan struct{} {
	return t.messageSolidSyncEvent.RegisterEvent(messageID.MapKey())
}

// DeregisterMessageSolidEvent removes a registed event to free the memory if not used.
func (t *Tangle) DeregisterMessageSolidEvent(messageID *hornet.MessageID) {
	t.messageSolidSyncEvent.DeregisterEvent(messageID.MapKey())
}

// RegisterMilestoneConfirmedEvent returns a channel that gets closed when the milestone is confirmed.
func (t *Tangle) RegisterMilestoneConfirmedEvent(msIndex milestone.Index) chan struct{} {
	return t.milestoneConfirmedSyncEvent.RegisterEvent(msIndex)
}

// DeregisterMilestoneConfirmedEvent removes a registed event to free the memory if not used.
func (t *Tangle) DeregisterMilestoneConfirmedEvent(msIndex milestone.Index) {
	t.milestoneConfirmedSyncEvent.DeregisterEvent(msIndex)
}

func (t *Tangle) PrintStatus() {
	var currentLowestMilestoneIndexInReqQ milestone.Index
	if peekedRequest := t.requestQueue.Peek(); peekedRequest != nil {
		currentLowestMilestoneIndexInReqQ = peekedRequest.MilestoneIndex
	}

	queued, pending, processing := t.requestQueue.Size()
	avgLatency := t.requestQueue.AvgLatency()

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
			t.receiveMsgWorkerPool.GetPendingQueueSize(),
			t.storage.GetSolidMilestoneIndex(),
			t.storage.GetLatestMilestoneIndex(),
			t.lastIncomingMPS,
			t.lastNewMPS,
			t.lastOutgoingMPS,
			t.serverMetrics.TipsNonLazy.Load(),
			t.serverMetrics.TipsSemiLazy.Load()))
}
