package tangle

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hive.go/core/math"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrDivisionByZero = errors.New("division by zero")
)

type ConfirmedMilestoneMetric struct {
	MilestoneIndex         iotago.MilestoneIndex
	BPS                    float64
	RBPS                   float64
	ReferencedRate         float64
	TimeSinceLastMilestone float64
}

// TriggerSolidifier can be used to manually trigger the solidifier from other plugins.
func (t *Tangle) TriggerSolidifier() {
	t.milestoneSolidifierWorkerPool.TrySubmit(SolidifierTriggerSignal, true)
}

func (t *Tangle) markBlockAsSolid(cachedBlockMeta *storage.CachedMetadata) {
	defer cachedBlockMeta.Release(true) // meta -1

	// update the solidity flags of this block
	cachedBlockMeta.Metadata().SetSolid(true)

	t.Events.BlockSolid.Trigger(cachedBlockMeta)
	t.blockSolidSyncEvent.Trigger(cachedBlockMeta.Metadata().BlockID())
}

// SolidQueueCheck traverses a milestone and checks if it is solid.
// Missing blocks are requested.
// Can be aborted if the given context is canceled.
func (t *Tangle) SolidQueueCheck(
	ctx context.Context,
	memcachedTraverserStorage dag.TraverserStorage,
	milestoneIndex iotago.MilestoneIndex,
	parents iotago.BlockIDs) (solid bool, aborted bool) {

	ts := time.Now()

	blocksChecked := 0
	var blockIDsToSolidify iotago.BlockIDs
	blockIDsToRequest := make(map[iotago.BlockID]struct{})

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// collect all block to solidify by traversing the tangle
	if err := parentsTraverser.Traverse(
		ctx,
		parents,
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			// if the block is solid, there is no need to traverse its parents
			return !cachedBlockMeta.Metadata().IsSolid(), nil
		},
		// consumer
		func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			// mark the block as checked
			blocksChecked++

			// collect the txToSolidify in an ordered way
			blockIDsToSolidify = append(blockIDsToSolidify, cachedBlockMeta.Metadata().BlockID())

			return nil
		},
		// called on missing parents
		func(parentBlockID iotago.BlockID) error {
			// block does not exist => request missing block
			blockIDsToRequest[parentBlockID] = struct{}{}

			return nil
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return false, true
		}
		t.LogPanic(err)
	}

	tCollect := time.Now()

	if len(blockIDsToRequest) > 0 {
		blockIDs := iotago.BlockIDs{}
		for blockID := range blockIDsToRequest {
			blockIDs = append(blockIDs, blockID)
		}
		requested := t.requester.RequestMultiple(blockIDs, milestoneIndex, true)
		t.LogWarnf("Stopped solidifier due to missing block -> Requested missing blocks (%d/%d), collect: %v", requested, len(blockIDs), tCollect.Sub(ts).Truncate(time.Millisecond))

		return false, false
	}

	// no blocks to request => the whole cone is solid
	// we mark all blocks as solid in order from oldest to latest (needed for the tip pool)
	for _, blockID := range blockIDsToSolidify {
		cachedBlockMeta, err := memcachedTraverserStorage.CachedBlockMetadata(blockID)
		if err != nil {
			t.LogPanicf("solidQueueCheck: Get block metadata failed: %v, Error: %w", blockID.ToHex(), err)

			return
		}
		if cachedBlockMeta == nil {
			t.LogPanicf("solidQueueCheck: Block metadata not found: %v", blockID.ToHex())

			return
		}

		t.markBlockAsSolid(cachedBlockMeta.Retain()) // meta pass +1
		cachedBlockMeta.Release(true)                // meta -1
	}

	tSolid := time.Now()

	if t.syncManager.IsNodeAlmostSynced() {
		// propagate solidity to the future cone (blocks attached to the blocks of this milestone)
		if err := t.futureConeSolidifier.SolidifyFutureConesWithMetadataMemcache(ctx, memcachedTraverserStorage, blockIDsToSolidify); err != nil {
			t.LogDebugf("SolidifyFutureConesWithMetadataMemcache failed: %s", err)
		}
	}

	t.LogInfof("Solidifier finished: blocks: %d, collect: %v, solidity %v, propagation: %v, total: %v", blocksChecked, tCollect.Sub(ts).Truncate(time.Millisecond), tSolid.Sub(tCollect).Truncate(time.Millisecond), time.Since(tSolid).Truncate(time.Millisecond), time.Since(ts).Truncate(time.Millisecond))

	return true, false
}

func (t *Tangle) newMilestoneSolidificationCtx() (context.Context, context.CancelFunc) {
	t.milestoneSolidificationCtxLock.Lock()
	defer t.milestoneSolidificationCtxLock.Unlock()

	// milestone solidification can be canceled by node shutdown or external signal
	ctx, ctxCancel := context.WithCancel(t.shutdownCtx)
	t.milestoneSolidificationCancelFunc = ctxCancel

	return ctx, ctxCancel
}

func (t *Tangle) AbortMilestoneSolidification() {
	t.milestoneSolidificationCtxLock.Lock()
	defer t.milestoneSolidificationCtxLock.Unlock()

	if t.milestoneSolidificationCancelFunc != nil {
		// cancel ongoing solidifications
		t.milestoneSolidificationCancelFunc()
		t.milestoneSolidificationCancelFunc = nil
	}
}

// solidifyMilestone tries to solidify the next known non-solid milestone and requests missing block.
func (t *Tangle) solidifyMilestone(newMilestoneIndex iotago.MilestoneIndex, force bool) {

	/* How milestone solidification works:

	- A Milestone comes in and gets validated
	- Request milestone parent1/parent2 without traversion
	- Everytime a request queue gets empty, start the solidifier for the next known non-solid milestone
	- If block are missing, they are requested by the solidifier
	- The traversion can be aborted with a signal and restarted
	*/
	if !force {
		/*
			If solidification is not forced, we will only run the solidifier under one of the following conditions:
				- newMilestoneIndex==0 (triggersignal) and solidifierMilestoneIndex==0 (no ongoing solidification)
				- newMilestoneIndex==solidMilestoneIndex+1 (next milestone)
				- newMilestoneIndex!=0 (new milestone received) and solidifierMilestoneIndex!=0 (ongoing solidification) and newMilestoneIndex<solidifierMilestoneIndex (milestone older than ongoing solidification)
				- newMilestoneIndex!=0 (new milestone received) and solidifierMilestoneIndex==0 (no ongoing solidification) and RequestQueue().Empty() (request queue is already empty)

			The following events trigger the solidifier in the node:
				- new valid milestone was processed (newMilestoneIndex=index, force=false)
				- a milestone was missing in the cone at solidifier run (newMilestoneIndex=0, force=true)
				- WebAPI call (newMilestoneIndex=0, force=true)
				- milestones in warp sync range were already in database at warpsync startup (newMilestoneIndex==0, force=true)
				- another milestone was successfully solidified (newMilestoneIndex=0, force=false)
				- request queue gets empty and node is not synced (newMilestoneIndex=0, force=true)
		*/

		t.solidifierMilestoneIndexLock.RLock()
		triggerSignal := (newMilestoneIndex == 0) && (t.solidifierMilestoneIndex == 0)
		nextMilestoneSignal := newMilestoneIndex == t.syncManager.ConfirmedMilestoneIndex()+1
		olderMilestoneDetected := (newMilestoneIndex != 0) && ((t.solidifierMilestoneIndex != 0) && (newMilestoneIndex < t.solidifierMilestoneIndex))
		newMilestoneReqQueueEmptySignal := (t.solidifierMilestoneIndex == 0) && (newMilestoneIndex != 0) && t.requestQueue.Empty()
		if !(triggerSignal || nextMilestoneSignal || olderMilestoneDetected || newMilestoneReqQueueEmptySignal) {
			// Do not run solidifier
			t.solidifierMilestoneIndexLock.RUnlock()

			return
		}
		t.solidifierMilestoneIndexLock.RUnlock()
	}

	// Stop possible other newer solidifications
	t.AbortMilestoneSolidification()

	t.solidifierLock.Lock()
	defer t.solidifierLock.Unlock()

	syncState := t.syncManager.SyncState()
	currentConfirmedIndex := syncState.ConfirmedMilestoneIndex
	latestIndex := syncState.LatestMilestoneIndex

	if currentConfirmedIndex == latestIndex && latestIndex != 0 {
		// Latest milestone already solid
		return
	}

	// always traverse the oldest non-solid milestone, either it gets solid, or something is missing that should be requested.
	milestoneIndexToSolidify, err := t.milestoneManager.FindClosestNextMilestoneIndex(currentConfirmedIndex)
	if err != nil {
		// No newer milestone available
		return
	}

	cachedMilestoneToSolidify := t.storage.CachedMilestoneByIndexOrNil(milestoneIndexToSolidify)
	if cachedMilestoneToSolidify == nil {
		// Milestone not found
		t.LogPanic(storage.ErrMilestoneNotFound)

		return
	}

	// Release shouldn't be forced, to cache the latest milestones
	defer cachedMilestoneToSolidify.Release() // milestone -1

	milestonePayloadToSolidify := cachedMilestoneToSolidify.Milestone().Milestone()

	t.setSolidifierMilestoneIndex(milestoneIndexToSolidify)

	milestoneSolidificationCtx, milestoneSolidificationCancelFunc := t.newMilestoneSolidificationCtx()
	defer milestoneSolidificationCancelFunc()

	blocksMemcache := storage.NewBlocksMemcache(t.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(t.storage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(t.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all blocks at the end
		blocksMemcache.Cleanup(true)

		// Release all block metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	t.LogInfof("Run solidity check for Milestone (%d) ...", milestoneIndexToSolidify)
	if becameSolid, aborted := t.SolidQueueCheck(
		milestoneSolidificationCtx,
		memcachedTraverserStorage,
		milestoneIndexToSolidify,
		milestonePayloadToSolidify.Parents,
	); !becameSolid {
		if aborted {
			// check was aborted due to older milestones/other solidifier running
			t.LogInfof("Aborted solid queue check for milestone %d", milestoneIndexToSolidify)
		} else {
			// Milestone not solid yet and missing block were requested
			t.Events.MilestoneSolidificationFailed.Trigger(milestoneIndexToSolidify)
			t.LogInfof("Milestone couldn't be solidified! %d", milestoneIndexToSolidify)
		}
		t.setSolidifierMilestoneIndex(0)

		return
	}

	if (currentConfirmedIndex + 1) < milestoneIndexToSolidify {

		// Milestone is stable, but some Milestones are missing in between
		// => check if they were found, or search for them in the solidified cone
		milestoneIndexClosestNext, err := t.milestoneManager.FindClosestNextMilestoneIndex(currentConfirmedIndex)
		if err != nil {
			t.LogPanicf("Milestones missing between (%d) and (%d).", currentConfirmedIndex, milestoneIndexToSolidify)
		}

		if milestoneIndexClosestNext == milestoneIndexToSolidify {
			t.LogInfof("Milestones missing between (%d) and (%d). Search for missing milestones ...", currentConfirmedIndex, milestoneIndexClosestNext)

			// no Milestones found in between => search an older milestone in the solid cone
			if found, err := t.searchMissingMilestones(
				milestoneSolidificationCtx,
				currentConfirmedIndex,
				milestoneIndexClosestNext,
				cachedMilestoneToSolidify.Milestone().Parents(),
			); !found {
				if err != nil {
					// no milestones found => this should not happen!
					t.LogPanicf("Milestones missing between (%d) and (%d).", currentConfirmedIndex, milestoneIndexClosestNext)
				}
				t.LogInfof("Aborted search for missing milestones between (%d) and (%d).", currentConfirmedIndex, milestoneIndexClosestNext)
			}
		}
		// rerun to solidify the older one
		t.setSolidifierMilestoneIndex(0)
		t.milestoneSolidifierWorkerPool.TrySubmit(SolidifierTriggerSignal, true)

		return
	}

	// solidify the direct children of the milestone parents,
	// to eventually solidify all blocks that contained the milestone payload itself.
	// this is needed to trigger the solid event for the milestone block that is expected by the coordinator.
	if err := t.futureConeSolidifier.SolidifyDirectChildrenWithMetadataMemcache(
		milestoneSolidificationCtx,
		memcachedTraverserStorage,
		milestonePayloadToSolidify.Parents,
	); err != nil {
		t.LogWarnf("Aborted confirmation of milestone %d because solidification of direct children failed: %s", milestoneIndexToSolidify, err.Error())

		return
	}

	var (
		timeStart                             time.Time
		timeSetConfirmedMilestoneIndexStart   time.Time
		timeSetConfirmedMilestoneIndexEnd     time.Time
		timeUpdateConeRootIndexesEnd          time.Time
		timeConfirmedMilestoneIndexChangedEnd time.Time
		timeConfirmedMilestoneChangedStart    time.Time
		timeConfirmedMilestoneChangedEnd      time.Time
	)

	var newReceipt *iotago.ReceiptMilestoneOpt
	var newConfirmation *whiteflag.Confirmation

	snapshotInfo := t.storage.SnapshotInfo()
	if snapshotInfo == nil {
		t.LogPanic(common.ErrSnapshotInfoNotFound)

		return
	}

	timeStart = time.Now()
	confirmedMilestoneStats, confirmationMetrics, err := whiteflag.ConfirmMilestone(
		t.storage.UTXOManager(),
		memcachedTraverserStorage,
		blocksMemcache.CachedBlock,
		t.protocolManager.Current(),
		snapshotInfo.GenesisMilestoneIndex(),
		milestonePayloadToSolidify,
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		t.serverMetrics,
		// Hint: Ledger is write locked
		func(rt *utxo.ReceiptTuple) error {
			newReceipt = rt.Receipt

			if t.receiptService != nil {
				if t.receiptService.ValidationEnabled {
					if err := t.receiptService.ValidateWithoutLocking(rt.Receipt); err != nil {
						if err := common.IsSoftError(err); err != nil && t.receiptService.IgnoreSoftErrors {
							t.LogWarnf("soft error encountered during receipt validation: %s", err)

							return nil
						}

						return err
					}
				}
				if t.receiptService.BackupEnabled {
					if err := t.receiptService.Backup(rt); err != nil {
						return fmt.Errorf("unable to confirm milestone due to receipt backup failure: %w", err)
					}
				}
			}

			return nil
		},
		// Hint: Ledger is write locked
		func(confirmation *whiteflag.Confirmation) {
			newConfirmation = confirmation

			timeSetConfirmedMilestoneIndexStart = time.Now()
			if err := t.syncManager.SetConfirmedMilestoneIndex(milestoneIndexToSolidify); err != nil {
				t.LogPanicf("SetConfirmedMilestoneIndex failed: %s", err)
			}
			timeSetConfirmedMilestoneIndexEnd = time.Now()

			if syncState.NodeAlmostSynced {
				// propagate new cone root indexes to the future cone (needed for URTS, heaviest branch tipselection, block broadcasting, etc...)
				// we can safely ignore errors of the future cone solidifier.
				_ = dag.UpdateConeRootIndexes(milestoneSolidificationCtx, memcachedTraverserStorage, confirmation.Mutations.ReferencedBlocks.BlockIDs(), confirmation.MilestoneIndex)
			}
			timeUpdateConeRootIndexesEnd = time.Now()

			t.Events.ConfirmedMilestoneIndexChanged.Trigger(milestoneIndexToSolidify)
			timeConfirmedMilestoneIndexChangedEnd = time.Now()
		},
		// Hint: Ledger is not locked
		func(blockMeta *storage.CachedMetadata, index iotago.MilestoneIndex, confTime uint32) {
			t.Events.BlockReferenced.Trigger(blockMeta, index, confTime)
		},
		// Hint: Ledger is not locked
		func(index iotago.MilestoneIndex, newOutputs utxo.Outputs, newSpents utxo.Spents) {
			t.Events.LedgerUpdated.Trigger(index, newOutputs, newSpents)
		},
		// Hint: Ledger is not locked
		func(index iotago.MilestoneIndex, tuple *utxo.TreasuryMutationTuple) {
			t.Events.TreasuryMutated.Trigger(index, tuple)
		})

	if err != nil {
		t.LogPanic(err)
	}

	if newReceipt != nil {
		t.Events.NewReceipt.Trigger(newReceipt)
	}

	timeConfirmedMilestoneChangedStart = time.Now()
	t.Events.ConfirmedMilestoneChanged.Trigger(cachedMilestoneToSolidify) // milestone pass +1
	timeConfirmedMilestoneChangedEnd = time.Now()

	if newConfirmation != nil {
		t.Events.ReferencedBlocksCountUpdated.Trigger(milestoneIndexToSolidify, len(newConfirmation.Mutations.ReferencedBlocks))
	}

	t.LogInfof("Milestone confirmed (%d): txsReferenced: %v, txsValue: %v, txsZeroValue: %v, txsConflicting: %v, collect: %v, total: %v",
		confirmedMilestoneStats.Index,
		confirmedMilestoneStats.BlocksReferenced,
		confirmedMilestoneStats.BlocksIncludedWithTransactions,
		confirmedMilestoneStats.BlocksExcludedWithoutTransactions,
		confirmedMilestoneStats.BlocksExcludedWithConflictingTransactions,
		confirmationMetrics.DurationWhiteflag.Truncate(time.Millisecond),
		time.Since(timeStart).Truncate(time.Millisecond),
	)

	confirmationMetrics.DurationSetConfirmedMilestoneIndex = timeSetConfirmedMilestoneIndexEnd.Sub(timeSetConfirmedMilestoneIndexStart)
	confirmationMetrics.DurationUpdateConeRootIndexes = timeUpdateConeRootIndexesEnd.Sub(timeSetConfirmedMilestoneIndexEnd)
	confirmationMetrics.DurationConfirmedMilestoneIndexChanged = timeConfirmedMilestoneIndexChangedEnd.Sub(timeUpdateConeRootIndexesEnd)
	confirmationMetrics.DurationConfirmedMilestoneChanged = timeConfirmedMilestoneChangedEnd.Sub(timeConfirmedMilestoneChangedStart)
	confirmationMetrics.DurationTotal = time.Since(timeStart)

	t.Events.ConfirmationMetricsUpdated.Trigger(confirmationMetrics)

	var rbpsBlock string
	if metric, err := t.calcConfirmedMilestoneMetric(milestonePayloadToSolidify); err == nil {
		isNodeSynced := t.syncManager.IsNodeSynced()
		if isNodeSynced {
			// Only trigger the metrics event if the node is sync (otherwise the BPS and conf.rate is wrong)
			if t.firstSyncedMilestone == 0 {
				t.firstSyncedMilestone = milestoneIndexToSolidify
			}
		} else {
			// reset the variable if unsynced
			t.firstSyncedMilestone = 0
		}

		if isNodeSynced && (milestoneIndexToSolidify > t.firstSyncedMilestone+1) {
			t.lastConfirmedMilestoneMetricLock.Lock()
			t.lastConfirmedMilestoneMetric = metric
			t.lastConfirmedMilestoneMetricLock.Unlock()

			// Ignore the first two milestones after node was sync (otherwise the BPS and conf.rate is wrong)
			rbpsBlock = fmt.Sprintf(", %0.2f BPS, %0.2f RBPS, %0.2f%% ref.rate", metric.BPS, metric.RBPS, metric.ReferencedRate)
		} else {
			rbpsBlock = fmt.Sprintf(", %0.2f RBPS", metric.RBPS)
		}
	}

	t.LogInfof("New confirmed milestone: %d%s", confirmedMilestoneStats.Index, rbpsBlock)

	// Run check for next milestone
	t.setSolidifierMilestoneIndex(0)

	if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		// do not trigger the next solidification if the node was shut down
		return
	}

	t.milestoneSolidifierWorkerPool.TrySubmit(SolidifierTriggerSignal, false)
}

func (t *Tangle) calcConfirmedMilestoneMetric(milestonePayloadToSolidify *iotago.Milestone) (*ConfirmedMilestoneMetric, error) {

	index := milestonePayloadToSolidify.Index

	timestampNew := milestonePayloadToSolidify.Timestamp
	timestampOld, err := t.storage.MilestoneTimestampUnixByIndex(index - 1)
	if err != nil {
		// milestone not found
		return nil, err
	}

	timeDiff := timestampNew - timestampOld
	if timeDiff == 0 {
		return nil, ErrDivisionByZero
	}

	timeDiffFloat := float64(timeDiff)

	newNewBlocksCount := t.serverMetrics.NewBlocks.Load()
	newBlocksDiff := math.Uint32Diff(newNewBlocksCount, t.oldNewBlocksCount)
	t.oldNewBlocksCount = newNewBlocksCount

	newReferencedBlocksCount := t.serverMetrics.ReferencedBlocks.Load()
	referencedBlocksDiff := math.Uint32Diff(newReferencedBlocksCount, t.oldReferencedBlocksCount)
	t.oldReferencedBlocksCount = newReferencedBlocksCount

	referencedRate := 0.0
	if newBlocksDiff != 0 {
		referencedRate = (float64(referencedBlocksDiff) / float64(newBlocksDiff)) * 100.0
	}

	metric := &ConfirmedMilestoneMetric{
		MilestoneIndex:         index,
		BPS:                    float64(newBlocksDiff) / timeDiffFloat,
		RBPS:                   float64(referencedBlocksDiff) / timeDiffFloat,
		ReferencedRate:         referencedRate,
		TimeSinceLastMilestone: timeDiffFloat,
	}

	return metric, nil
}

func (t *Tangle) setSolidifierMilestoneIndex(index iotago.MilestoneIndex) {
	t.solidifierMilestoneIndexLock.Lock()
	t.solidifierMilestoneIndex = index
	t.solidifierMilestoneIndexLock.Unlock()
}

// searchMissingMilestones searches milestones in the cone that are not persisted in the DB yet by traversing the tangle.
func (t *Tangle) searchMissingMilestones(ctx context.Context, confirmedMilestoneIndex iotago.MilestoneIndex, startMilestoneIndex iotago.MilestoneIndex, milestoneParents iotago.BlockIDs) (found bool, err error) {

	var milestoneFound bool

	ts := time.Now()

	if err := dag.TraverseParents(
		ctx,
		t.storage,
		milestoneParents,
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			// if the block is referenced by an older milestone, there is no need to traverse its parents
			if referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex(); referenced && (at <= confirmedMilestoneIndex) {
				return false, nil
			}

			blockID := cachedBlockMeta.Metadata().BlockID()
			cachedBlock := t.storage.CachedBlockOrNil(blockID) // block +1
			if cachedBlock == nil {
				return false, fmt.Errorf("%w block ID: %s", common.ErrBlockNotFound, blockID.ToHex())
			}
			defer cachedBlock.Release(true) // block -1

			milestonePayload := t.milestoneManager.VerifyMilestoneBlock(cachedBlock.Block().Block())
			if milestonePayload == nil {
				return true, nil
			}

			msIndex := milestonePayload.Index
			if (msIndex <= confirmedMilestoneIndex) || (msIndex >= startMilestoneIndex) {
				return true, nil
			}

			// milestone found!
			t.milestoneManager.StoreMilestone(cachedBlock.Retain(), milestonePayload, false) // block pass +1
			milestoneFound = true

			return true, nil // we keep searching for all missing milestones
		},
		// consumer
		nil,
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {

		if errors.Is(err, common.ErrOperationAborted) {
			return false, nil
		}

		return false, err
	}

	t.LogInfof("searchMissingMilestone finished, found: %v, took: %v", milestoneFound, time.Since(ts).Truncate(time.Millisecond))

	return milestoneFound, nil
}
