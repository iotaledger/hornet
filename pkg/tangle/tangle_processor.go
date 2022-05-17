package tangle

import (
	"context"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/contextutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/hive.go/workerpool"
)

func (t *Tangle) ConfigureTangleProcessor() {

	t.receiveBlockWorkerPool = workerpool.New(func(task workerpool.Task) {
		t.processIncomingTx(task.Param(0).(*storage.Block), task.Param(1).(gossip.Requests), task.Param(2).(*gossip.Protocol))
		task.Return(nil)
	}, workerpool.WorkerCount(t.receiveBlockWorkerCount), workerpool.QueueSize(t.receiveBlockQueueSize))

	t.futureConeSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		if err := t.futureConeSolidifier.SolidifyBlockAndFutureCone(t.shutdownCtx, task.Param(0).(*storage.CachedMetadata)); err != nil {
			t.LogDebugf("SolidifyBlockAndFutureCone failed: %s", err)
		}
		task.Return(nil)
	}, workerpool.WorkerCount(t.futureConeSolidifierWorkerCount), workerpool.QueueSize(t.futureConeSolidifierQueueSize), workerpool.FlushTasksAtShutdown(true))

	t.processValidMilestoneWorkerPool = workerpool.New(func(task workerpool.Task) {
		t.processValidMilestone(task.Param(0).(hornet.BlockID), task.Param(1).(*storage.CachedMilestone), task.Param(2).(bool)) // milestone pass +1
		task.Return(nil)
	}, workerpool.WorkerCount(t.processValidMilestoneWorkerCount), workerpool.QueueSize(t.processValidMilestoneQueueSize), workerpool.FlushTasksAtShutdown(true))

	t.milestoneSolidifierWorkerPool = workerpool.New(func(task workerpool.Task) {
		t.solidifyMilestone(task.Param(0).(milestone.Index), task.Param(1).(bool))
		task.Return(nil)
	}, workerpool.WorkerCount(t.milestoneSolidifierWorkerCount), workerpool.QueueSize(t.milestoneSolidifierQueueSize))
}

func (t *Tangle) RunTangleProcessor() {
	t.LogInfo("Starting TangleProcessor ...")

	// set latest known milestone from database
	latestMilestoneFromDatabase := t.storage.SearchLatestMilestoneIndexInStore()
	if latestMilestoneFromDatabase < t.syncManager.ConfirmedMilestoneIndex() {
		latestMilestoneFromDatabase = t.syncManager.ConfirmedMilestoneIndex()
	}

	t.syncManager.SetLatestMilestoneIndex(latestMilestoneFromDatabase, t.updateSyncedAtStartup)

	t.startWaitGroup.Add(5)

	onBlockProcessed := events.NewClosure(func(block *storage.Block, requests gossip.Requests, proto *gossip.Protocol) {
		t.receiveBlockWorkerPool.Submit(block, requests, proto)
	})

	onLatestMilestoneIndexChanged := events.NewClosure(func(_ milestone.Index) {
		// cleanup the future cone solidifier to free the caches
		t.futureConeSolidifier.Cleanup(true)

		// reset the milestone timeout ticker
		t.ResetMilestoneTimeoutTicker()
	})

	onMilestoneTimeout := events.NewClosure(func() {
		// cleanup the future cone solidifier on milestone timeouts to free the caches
		t.futureConeSolidifier.Cleanup(true)
	})

	onBPSMetricsUpdated := events.NewClosure(func(bpsMetrics *BPSMetrics) {
		t.lastIncomingBPS = bpsMetrics.Incoming
		t.lastNewBPS = bpsMetrics.New
		t.lastOutgoingBPS = bpsMetrics.Outgoing
	})

	// send all solid blocks back to the block processor, which broadcasts them to other nodes
	// after passing some additional rules.
	onBlockSolid := events.NewClosure(func(cachedBlockMeta *storage.CachedMetadata) {
		t.messageProcessor.Broadcast(cachedBlockMeta) // meta pass +1
	})

	onReceivedValidMilestone := events.NewClosure(func(blockID hornet.BlockID, cachedMilestone *storage.CachedMilestone, requested bool) {

		if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
			// do not process the milestone if the node was shut down
			cachedMilestone.Release(true) // milestone -1
			return
		}

		_, added := t.processValidMilestoneWorkerPool.Submit(blockID, cachedMilestone, requested) // milestone pass +1
		if !added {
			// Release shouldn't be forced, to cache the latest milestones
			cachedMilestone.Release() // milestone -1
		}
	})

	// create a background worker that "measures" the BPS value every second
	if err := t.daemon.BackgroundWorker("Metrics BPS Updater", func(ctx context.Context) {
		ticker := timeutil.NewTicker(t.measureBPS, 1*time.Second, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PriorityMetricsUpdater); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[UpdateMetrics]", func(ctx context.Context) {
		t.Events.BPSMetricsUpdated.Attach(onBPSMetricsUpdated)
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.Events.BPSMetricsUpdated.Detach(onBPSMetricsUpdated)
	}, daemon.PriorityMetricsUpdater); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(ctx context.Context) {
		t.LogInfo("Starting TangleProcessor[ReceiveTx] ... done")
		t.messageProcessor.Events.BlockProcessed.Attach(onBlockProcessed)
		t.Events.BlockSolid.Attach(onBlockSolid)
		t.receiveBlockWorkerPool.Start()
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.LogInfo("Stopping TangleProcessor[ReceiveTx] ...")
		t.messageProcessor.Events.BlockProcessed.Detach(onBlockProcessed)
		t.Events.BlockSolid.Detach(onBlockSolid)
		t.receiveBlockWorkerPool.StopAndWait()
		t.LogInfo("Stopping TangleProcessor[ReceiveTx] ... done")
	}, daemon.PriorityReceiveTxWorker); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[FutureConeSolidifier]", func(ctx context.Context) {
		t.LogInfo("Starting TangleProcessor[FutureConeSolidifier] ... done")
		t.futureConeSolidifierWorkerPool.Start()
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.LogInfo("Stopping TangleProcessor[FutureConeSolidifier] ...")
		t.futureConeSolidifierWorkerPool.StopAndWait()
		t.LogInfo("Stopping TangleProcessor[FutureConeSolidifier] ... done")
	}, daemon.PrioritySolidifierGossip); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(ctx context.Context) {
		t.LogInfo("Starting TangleProcessor[ProcessMilestone] ... done")
		t.processValidMilestoneWorkerPool.Start()
		t.milestoneManager.Events.ReceivedValidMilestone.Attach(onReceivedValidMilestone)
		t.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		t.Events.MilestoneTimeout.Attach(onMilestoneTimeout)
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.LogInfo("Stopping TangleProcessor[ProcessMilestone] ...")
		t.StopMilestoneTimeoutTicker()
		t.milestoneManager.Events.ReceivedValidMilestone.Detach(onReceivedValidMilestone)
		t.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
		t.Events.MilestoneTimeout.Detach(onMilestoneTimeout)
		t.processValidMilestoneWorkerPool.StopAndWait()
		t.LogInfo("Stopping TangleProcessor[ProcessMilestone] ... done")
	}, daemon.PriorityMilestoneProcessor); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[MilestoneSolidifier]", func(ctx context.Context) {
		t.LogInfo("Starting TangleProcessor[MilestoneSolidifier] ... done")
		t.milestoneSolidifierWorkerPool.Start()
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.LogInfo("Stopping TangleProcessor[MilestoneSolidifier] ...")
		t.milestoneSolidifierWorkerPool.StopAndWait()
		t.futureConeSolidifier.Cleanup(true)
		t.LogInfo("Stopping TangleProcessor[MilestoneSolidifier] ... done")
	}, daemon.PriorityMilestoneSolidifier); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

}

// WaitForTangleProcessorStartup waits until all background workers of the tangle processor are started.
func (t *Tangle) WaitForTangleProcessorStartup() {
	t.startWaitGroup.Wait()
}

func (t *Tangle) IsReceiveTxWorkerPoolBusy() bool {
	return t.receiveBlockWorkerPool.GetPendingQueueSize() > (t.receiveBlockQueueSize / 2)
}

func (t *Tangle) processIncomingTx(incomingBlock *storage.Block, requests gossip.Requests, proto *gossip.Protocol) {

	latestMilestoneIndex := t.syncManager.LatestMilestoneIndex()
	isNodeSyncedWithinBelowMaxDepth := t.syncManager.IsNodeSyncedWithinBelowMaxDepth()

	requested := requests.HasRequest()

	// The block will be added to the storage inside this function, so the block object automatically updates
	cachedBlock, alreadyAdded := AddBlockToStorage(t.storage, t.milestoneManager, incomingBlock, latestMilestoneIndex, requested, !isNodeSyncedWithinBelowMaxDepth) // block +1

	// Release shouldn't be forced, to cache the latest blocks
	defer cachedBlock.Release(!isNodeSyncedWithinBelowMaxDepth) // block -1

	if !alreadyAdded {
		t.serverMetrics.NewBlocks.Inc()

		if proto != nil {
			proto.Metrics.NewBlocks.Inc()
		}

		// since we only add the parents if there was a source request, we only
		// request them for blocks which should be part of milestone cones
		for _, request := range requests {
			// add this newly received block's parents to the request queue
			if request.RequestType == gossip.RequestTypeBlockID {
				t.requester.RequestParents(cachedBlock.Retain(), request.MilestoneIndex, true) // block pass +1
			}
		}

		confirmedMilestoneIndex := t.syncManager.ConfirmedMilestoneIndex()
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = confirmedMilestoneIndex
		}

		if t.syncManager.IsNodeAlmostSynced() {
			// try to solidify the block and its future cone
			t.futureConeSolidifierWorkerPool.Submit(cachedBlock.CachedMetadata()) // meta pass +1
		}

		t.Events.ReceivedNewBlock.Trigger(cachedBlock, latestMilestoneIndex, confirmedMilestoneIndex)

	} else {
		t.serverMetrics.KnownBlocks.Inc()
		if proto != nil {
			proto.Metrics.KnownBlocks.Inc()
		}
		t.Events.ReceivedKnownBlock.Trigger(cachedBlock)
	}

	// "ProcessedBlock" event has to be fired after "ReceivedNewBlock" event,
	// otherwise there is a race condition in the coordinator plugin that tries to "ComputeMerkleTreeRootHash"
	// with the block it issued itself because the block may be not solid yet and therefore their database entries
	// are not created yet.
	t.Events.ProcessedBlock.Trigger(incomingBlock.BlockID())
	t.blockProcessedSyncEvent.Trigger(incomingBlock.BlockID().ToMapKey())

	for _, request := range requests {
		// mark the received request as processed
		t.requestQueue.Processed(request)
	}

	// we check whether the request is nil, so we only trigger the solidifier when
	// we actually handled a block coming from a request (as otherwise the solidifier
	// is triggered too often through blocks received from normal gossip)
	if requested && !t.syncManager.IsNodeSynced() && t.requestQueue.Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		t.milestoneSolidifierWorkerPool.TrySubmit(milestone.Index(0), true)
	}
}

// RegisterBlockProcessedEvent returns a channel that gets closed when the block is processed.
func (t *Tangle) RegisterBlockProcessedEvent(blockID hornet.BlockID) chan struct{} {
	return t.blockProcessedSyncEvent.RegisterEvent(blockID.ToMapKey())
}

// DeregisterBlockProcessedEvent removes a registered event to free the memory if not used.
func (t *Tangle) DeregisterBlockProcessedEvent(blockID hornet.BlockID) {
	t.blockProcessedSyncEvent.DeregisterEvent(blockID.ToMapKey())
}

// RegisterBlockSolidEvent returns a channel that gets closed when the block is marked as solid.
func (t *Tangle) RegisterBlockSolidEvent(blockID hornet.BlockID) chan struct{} {
	return t.blockSolidSyncEvent.RegisterEvent(blockID.ToMapKey())
}

// DeregisterBlockSolidEvent removes a registered event to free the memory if not used.
func (t *Tangle) DeregisterBlockSolidEvent(blockID hornet.BlockID) {
	t.blockSolidSyncEvent.DeregisterEvent(blockID.ToMapKey())
}

// RegisterMilestoneConfirmedEvent returns a channel that gets closed when the milestone is confirmed.
func (t *Tangle) RegisterMilestoneConfirmedEvent(msIndex milestone.Index) chan struct{} {
	return t.milestoneConfirmedSyncEvent.RegisterEvent(msIndex)
}

// DeregisterMilestoneConfirmedEvent removes a registered event to free the memory if not used.
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
				"CMI/LMI: %d/%d, "+
				"BPS (in/new/out): %05d/%05d/%05d, "+
				"Tips (non-/semi-lazy): %d/%d",
			queued, pending, processing, avgLatency,
			currentLowestMilestoneIndexInReqQ,
			t.receiveBlockWorkerPool.GetPendingQueueSize(),
			t.syncManager.ConfirmedMilestoneIndex(),
			t.syncManager.LatestMilestoneIndex(),
			t.lastIncomingBPS,
			t.lastNewBPS,
			t.lastOutgoingBPS,
			t.serverMetrics.TipsNonLazy.Load(),
			t.serverMetrics.TipsSemiLazy.Load()))
}
