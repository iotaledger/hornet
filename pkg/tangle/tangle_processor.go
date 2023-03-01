package tangle

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/contextutils"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/timeutil"
	"github.com/iotaledger/hive.go/runtime/valuenotifier"
	"github.com/iotaledger/hive.go/runtime/workerpool"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	SolidifierTriggerSignal = iotago.MilestoneIndex(0)
)

func (t *Tangle) ConfigureTangleProcessor() {
	t.receiveBlockWorkerPool = workerpool.New("ReceiveBlock", t.receiveBlockWorkerCount)
	t.futureConeSolidifierWorkerPool = workerpool.New("FutureConeSolidifier", t.futureConeSolidifierWorkerCount)
	t.processValidMilestoneWorkerPool = workerpool.New("ProcessValidMilestone", t.processValidMilestoneWorkerCount)
	t.milestoneSolidifierWorkerPool = workerpool.New("MilestoneSolidifier", t.milestoneSolidifierWorkerCount)
}

func (t *Tangle) RunTangleProcessor() {
	t.LogInfo("Starting TangleProcessor ...")

	// set latest known milestone from database
	latestMilestoneFromDatabase := t.storage.SearchLatestMilestoneIndexInStore()
	confirmedMilestoneIndex := t.syncManager.ConfirmedMilestoneIndex()
	if latestMilestoneFromDatabase < confirmedMilestoneIndex {
		latestMilestoneFromDatabase = confirmedMilestoneIndex
	}

	t.syncManager.SetLatestMilestoneIndex(latestMilestoneFromDatabase, t.updateSyncedAtStartup)

	t.startWaitGroup.Add(5)

	// create a background worker that "measures" the BPS value every second
	if err := t.daemon.BackgroundWorker("Metrics BPS Updater", func(ctx context.Context) {
		ticker := timeutil.NewTicker(t.measureBPS, 1*time.Second, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PriorityMetricsUpdater); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[UpdateMetrics]", func(ctx context.Context) {
		unhook := t.Events.BPSMetricsUpdated.Hook(func(bpsMetrics *BPSMetrics) {
			t.lastIncomingBPS = bpsMetrics.Incoming
			t.lastNewBPS = bpsMetrics.New
			t.lastOutgoingBPS = bpsMetrics.Outgoing
		}).Unhook
		t.startWaitGroup.Done()
		<-ctx.Done()
		unhook()
	}, daemon.PriorityMetricsUpdater); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[ReceiveTx]", func(ctx context.Context) {
		t.LogInfo("Starting TangleProcessor[ReceiveTx] ... done")
		unhook := lo.Batch(
			t.messageProcessor.Events.BlockProcessed.Hook(t.processIncomingTx, event.WithWorkerPool(t.receiveBlockWorkerPool)).Unhook,

			// send all solid blocks back to the block processor, which broadcasts them to other nodes
			// after passing some additional rules.
			t.Events.BlockSolid.Hook(func(cachedBlockMeta *storage.CachedMetadata) {
				t.messageProcessor.Broadcast(cachedBlockMeta) // meta pass +1
			}).Unhook,
		)
		t.receiveBlockWorkerPool.Start()
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.LogInfo("Stopping TangleProcessor[ReceiveTx] ...")
		unhook()
		t.receiveBlockWorkerPool.Shutdown()
		t.receiveBlockWorkerPool.ShutdownComplete.Wait()
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
		t.futureConeSolidifierWorkerPool.Shutdown()
		t.futureConeSolidifierWorkerPool.ShutdownComplete.Wait()
		t.LogInfo("Stopping TangleProcessor[FutureConeSolidifier] ... done")
	}, daemon.PrioritySolidifierGossip); err != nil {
		t.LogPanicf("failed to start worker: %s", err)
	}

	if err := t.daemon.BackgroundWorker("TangleProcessor[ProcessMilestone]", func(ctx context.Context) {
		t.LogInfo("Starting TangleProcessor[ProcessMilestone] ... done")
		t.processValidMilestoneWorkerPool.Start()
		unhook := lo.Batch(
			t.milestoneManager.Events.ReceivedValidMilestone.Hook(func(cachedMilestone *storage.CachedMilestone, requested bool) {
				if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
					// do not process the milestone if the node was shut down
					cachedMilestone.Release(true) // milestone -1

					return
				}

				t.processValidMilestoneWorkerPool.Submit(func() {
					t.processValidMilestone(cachedMilestone, requested) // milestone pass +1
				})
			}).Unhook,

			t.Events.LatestMilestoneIndexChanged.Hook(func(_ iotago.MilestoneIndex) {
				// cleanup the future cone solidifier to free the caches
				t.futureConeSolidifier.Cleanup(true)

				// reset the milestone timeout ticker
				t.ResetMilestoneTimeoutTicker()
			}).Unhook,

			t.Events.MilestoneTimeout.Hook(func() {
				// cleanup the future cone solidifier on milestone timeouts to free the caches
				t.futureConeSolidifier.Cleanup(true)
			}).Unhook,
		)
		t.startWaitGroup.Done()
		<-ctx.Done()
		t.LogInfo("Stopping TangleProcessor[ProcessMilestone] ...")
		t.StopMilestoneTimeoutTicker()
		unhook()
		t.processValidMilestoneWorkerPool.Shutdown()
		t.processValidMilestoneWorkerPool.ShutdownComplete.Wait()
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
		t.milestoneSolidifierWorkerPool.Shutdown()
		t.milestoneSolidifierWorkerPool.ShutdownComplete.Wait()
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
	return t.receiveBlockWorkerPool.PendingTasksCounter.Get() > (t.receiveBlockQueueSize / 2)
}

func (t *Tangle) processIncomingTx(incomingBlock *storage.Block, requests gossip.Requests, proto *gossip.Protocol) {

	syncState := t.syncManager.SyncState()
	latestMilestoneIndex := syncState.LatestMilestoneIndex
	isNodeSyncedWithinBelowMaxDepth := syncState.NodeSyncedWithinBelowMaxDepth

	requested := requests.HasRequest()

	// The block will be added to the storage inside this function, so the block object automatically updates
	cachedBlock, alreadyAdded := AddBlockToStorage(t.storage, t.milestoneManager, incomingBlock, latestMilestoneIndex, requested, !isNodeSyncedWithinBelowMaxDepth) // block +1

	// Release shouldn't be forced, to cache the latest blocks
	defer cachedBlock.Release(!isNodeSyncedWithinBelowMaxDepth) // block -1

	if !alreadyAdded {
		t.serverMetrics.NewBlocks.Inc()

		// increase the new block metric for the peer that submitted the block
		if proto != nil {
			proto.Metrics.NewBlocks.Inc()
		}

		hadRequests := false

		// since we only add the parents if there was a source request, we only
		// request them for blocks which should be part of milestone cones
		for _, request := range requests {
			// add this newly received block's parents to the request queue
			if request.RequestType == gossip.RequestTypeBlockID {
				t.requester.RequestParents(cachedBlock.Retain(), request.MilestoneIndex, request.PreventDiscard) // block pass +1
				hadRequests = true
			}
		}

		if !hadRequests && syncState.NodeAlmostSynced && !t.resyncPhaseDone.Load() {
			// request parents of newly seen blocks during resync phase (requests may be discarded).
			// this is done to solidify blocks in the future cone during resync.
			t.requester.RequestParents(cachedBlock.Retain(), syncState.LatestMilestoneIndex, false) // block pass +1
		}

		confirmedMilestoneIndex := syncState.ConfirmedMilestoneIndex
		if latestMilestoneIndex == 0 {
			latestMilestoneIndex = confirmedMilestoneIndex
		}

		if syncState.NodeAlmostSynced {
			// we need to solidify the block before marking "blockProcessedSyncEvent" as done,
			// otherwise clients might successfully attach blocks to the node and reuse them as parents
			// in further transactions, knowing that these blocks are solid, but for the node itself they might not be solid yet,
			// because the asynchronous futureConeSolidifierWorkerPool did not process the block yet.
			if isSolid, newlySolid, err := checkBlockSolid(t.storage, cachedBlock.CachedMetadata()); err == nil { // meta pass +1
				if newlySolid {
					t.markBlockAsSolid(cachedBlock.CachedMetadata()) // meta pass +1
				}

				if isSolid {
					// try to solidify the future cone of the block
					cachedMeta := cachedBlock.CachedMetadata() // meta +1
					t.futureConeSolidifierWorkerPool.Submit(func() {
						if err := t.futureConeSolidifier.SolidifyBlockAndFutureCone(t.shutdownCtx, cachedMeta); err != nil {
							t.LogDebugf("SolidifyBlockAndFutureCone failed: %s", err)
						} // meta pass +1
					})
				}
			}
		}

		t.Events.ReceivedNewBlock.Trigger(cachedBlock, latestMilestoneIndex, confirmedMilestoneIndex)

	} else {
		t.serverMetrics.KnownBlocks.Inc()

		// increase the known block metric for the peer that submitted the block
		if proto != nil {
			proto.Metrics.KnownBlocks.Inc()
		}
	}

	t.blockProcessedNotifier.Notify(incomingBlock.BlockID())

	for _, request := range requests {
		// mark the received request as processed
		t.requestQueue.Processed(request)
	}

	// we check whether the request is nil, so we only trigger the solidifier when
	// we actually handled a block coming from a request (as otherwise the solidifier
	// is triggered too often through blocks received from normal gossip)
	if requested && !syncState.NodeSynced && t.requestQueue.Empty() {
		// we trigger the milestone solidifier in order to solidify milestones
		// which should be solid given that the request queue is empty
		t.milestoneSolidifierWorkerPool.Submit(func() {
			t.solidifyMilestone(SolidifierTriggerSignal, true)
		})
	}
}

// BlockProcessedListener returns a listener that can be used to wait until the block is processed.
func (t *Tangle) BlockProcessedListener(blockID iotago.BlockID) *valuenotifier.Listener {
	return t.blockProcessedNotifier.Listener(blockID)
}

// BlockSolidListener returns a listener that can be used to wait until a block is marked as solid.
func (t *Tangle) BlockSolidListener(blockID iotago.BlockID) *valuenotifier.Listener {
	return t.blockSolidNotifier.Listener(blockID)
}

func (t *Tangle) PrintStatus() {
	var currentLowestMilestoneIndexInReqQ iotago.MilestoneIndex
	if peekedRequest := t.requestQueue.Peek(); peekedRequest != nil {
		currentLowestMilestoneIndexInReqQ = peekedRequest.MilestoneIndex
	}

	queued, pending, processing := t.requestQueue.Size()
	avgLatency := t.requestQueue.AvgLatency()

	syncState := t.syncManager.SyncState()
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
			t.receiveBlockWorkerPool.PendingTasksCounter.Get(),
			syncState.ConfirmedMilestoneIndex,
			syncState.LatestMilestoneIndex,
			t.lastIncomingBPS,
			t.lastNewBPS,
			t.lastOutgoingBPS,
			t.serverMetrics.TipsNonLazy.Load(),
			t.serverMetrics.TipsSemiLazy.Load()))
}
