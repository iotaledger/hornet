package tangle

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// printStatusInterval is the interval for printing status blocks.
	printStatusInterval = 2 * time.Second
)

var (
	ErrLatestMilestoneOlderThanSnapshotIndex = errors.New("latest milestone in the database is older than the snapshot index")
	ErrSnapshotIndexWrong                    = errors.New("snapshot index does not fit the snapshot ledger index")
)

// RevalidateDatabase tries to revalidate a corrupted database (after an unclean node shutdown/crash)
//
// HORNET uses caches for almost all tangle related data.
// If the node crashes, it is not guaranteed that all data in the cache was already persisted to the disk.
// Thats why we flag the database as corrupted.
//
// This function tries to restore a clean database state by deleting all existing blocks
// since last local snapshot, deleting all ledger states and changes, and loading a valid snapshot ledger state.
//
// This way HORNET should be able to re-solidify the existing tangle in the database.
//
// Object Storages:
//   - Milestone							=> will be removed and added again if missing by receiving the block
//   - Block							=> will be removed and added again by requesting the block at solidification
//   - BlockMetadata   				=> will be removed and added again if missing by receiving the block
//   - Children							=> will be removed and added again if missing by receiving the block
//   - Indexation						=> will be removed and added again if missing by receiving the block
//   - UnreferencedBlock 				=> will be removed at pruning anyway
//
// Database:
//   - LedgerState
//   - Unspent						=> will be removed and loaded again from last snapshot
//   - Spent							=> will be removed and loaded again from last snapshot
//   - Balances						=> will be removed and loaded again from last snapshot
//   - Diffs							=> will be removed and loaded again from last snapshot
//   - Treasury						=> will be removed and loaded again from last snapshot
//   - Receipts						=> will be removed and loaded again from last snapshot (if pruneReceipts is enabled)
func (t *Tangle) RevalidateDatabase(snapshotImporter *snapshot.Importer, pruneReceipts bool) error {

	// mark the database as tainted forever.
	// this is used to signal the coordinator plugin that it should never use a revalidated database.
	if err := t.storage.MarkStoresTainted(); err != nil {
		t.LogPanic(err)
	}

	start := time.Now()

	snapshotInfo := t.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return common.ErrSnapshotInfoNotFound
	}

	latestMilestoneIndex := t.storage.SearchLatestMilestoneIndexInStore()

	if snapshotInfo.SnapshotIndex() > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	// check if the ledger index of the snapshot files fit the revalidation target.
	snapshotLedgerIndex, err := snapshotImporter.SnapshotsFilesLedgerIndex()
	if err != nil {
		return err
	}

	if snapshotLedgerIndex != snapshotInfo.SnapshotIndex() {
		return fmt.Errorf("snapshot files (index: %d) do not fit the revalidation target (index: %d)", snapshotLedgerIndex, snapshotInfo.SnapshotIndex())
	}

	t.LogInfof("reverting database state back from %d to snapshot %d (this might take a while) ... ", latestMilestoneIndex, snapshotInfo.SnapshotIndex())

	// deletes all ledger entries (unspent, spent, diffs, balances, treasury, receipts).
	if err := t.cleanupLedger(pruneReceipts); err != nil {
		return err
	}

	// delete milestone data newer than the local snapshot
	if err := t.cleanupMilestones(snapshotInfo); err != nil {
		return err
	}

	// deletes all blocks which metadata doesn't exist anymore, are not referenced, not solid or
	// their confirmation milestone is newer than the last local snapshot's milestone.
	if err := t.cleanupBlocks(snapshotInfo); err != nil {
		return err
	}

	// deletes all block metadata where the blocks doesn't exist in the database anymore.
	if err := t.cleanupBlockMetadata(); err != nil {
		return err
	}

	// deletes all children where the block metadata doesn't exist in the database anymore.
	if err := t.cleanupChildren(); err != nil {
		return err
	}

	// deletes all unreferenced blocks that are left in the database (we don't need them since we deleted all unreferenced blocks).
	if err := t.cleanupUnreferencedBlocks(); err != nil {
		return err
	}

	t.LogInfo("flushing storages ...")
	t.storage.FlushStorages()
	t.LogInfo("flushing storages ... done!")

	// apply the ledger from the last snapshot to the database
	if err := t.applySnapshotLedger(snapshotInfo, snapshotImporter); err != nil {
		return err
	}

	t.LogInfof("reverted state back to snapshot %d, took %v", snapshotInfo.SnapshotIndex(), time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all ledger entries (unspent, spent, diffs, balances, treasury, receipts).
func (t *Tangle) cleanupLedger(pruneReceipts bool) error {

	start := time.Now()

	t.LogInfo("clearing ledger ... ")
	if err := t.storage.UTXOManager().ClearLedger(pruneReceipts); err != nil {
		return err
	}
	t.LogInfof("clearing ledger ... done. took %v", time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func (t *Tangle) cleanupMilestones(info *storage.SnapshotInfo) error {

	start := time.Now()

	milestonesToDelete := make(map[iotago.MilestoneIndex]struct{})

	lastStatusTime := time.Now()
	var milestonesCounter int64
	t.storage.NonCachedStorage().ForEachMilestoneIndex(func(msIndex iotago.MilestoneIndex) bool {
		milestonesCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d milestones", milestonesCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex() {
			return true
		}

		milestonesToDelete[msIndex] = struct{}{}

		return true
	})

	if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(milestonesToDelete)
	var deletionCounter int64
	for msIndex := range milestonesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting milestones ... %d/%d (%0.2f%%). %v left ...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteUnreferencedBlocks(msIndex)
		t.storage.DeleteMilestone(msIndex)
	}

	t.storage.FlushUnreferencedBlocksStorage()
	t.storage.FlushMilestoneStorage()

	t.LogInfof("deleting milestones ... %d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all blocks which metadata doesn't exist anymore, are not referenced, not solid or
// their confirmation milestone is newer than the last local snapshot's milestone.
func (t *Tangle) cleanupBlocks(info *storage.SnapshotInfo) error {

	start := time.Now()

	blocksToDelete := make(map[iotago.BlockID]struct{})

	lastStatusTime := time.Now()
	var txsCounter int64
	t.storage.NonCachedStorage().ForEachBlockID(func(blockID iotago.BlockID) bool {
		txsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d blocks", txsCounter)
		}

		storedTxMeta := t.storage.StoredMetadataOrNil(blockID)

		// delete block if metadata doesn't exist
		if storedTxMeta == nil {
			blocksToDelete[blockID] = struct{}{}

			return true
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			blocksToDelete[blockID] = struct{}{}

			return true
		}

		// not referenced or above snapshot index
		if referenced, by := storedTxMeta.ReferencedWithIndex(); !referenced || by > info.SnapshotIndex() {
			blocksToDelete[blockID] = struct{}{}

			return true
		}

		return true
	})
	t.LogInfof("analyzed %d blocks", txsCounter)

	if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(blocksToDelete)
	var deletionCounter int64
	for blockID := range blocksToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting blocks ... %d/%d (%0.2f%%). %v left ...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteBlock(blockID)
	}

	t.storage.FlushBlocksStorage()

	t.LogInfof("deleting blocks ... %d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all block metadata where the block doesn't exist in the database anymore.
func (t *Tangle) cleanupBlockMetadata() error {

	start := time.Now()

	metadataToDelete := make(map[iotago.BlockID]struct{})

	lastStatusTime := time.Now()
	var metadataCounter int64
	t.storage.NonCachedStorage().ForEachBlockMetadataBlockID(func(blockID iotago.BlockID) bool {
		metadataCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d block metadata", metadataCounter)
		}

		// delete metadata if block doesn't exist
		if !t.storage.BlockExistsInStore(blockID) {
			metadataToDelete[blockID] = struct{}{}
		}

		return true
	})
	t.LogInfof("analyzed %d block metadata", metadataCounter)

	if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(metadataToDelete)
	var deletionCounter int64
	for blockID := range metadataToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting block metadata ... %d/%d (%0.2f%%). %v left ...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteBlockMetadata(blockID)
	}

	t.storage.FlushBlocksStorage()

	t.LogInfof("deleting block metadata ... %d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all children where the block metadata doesn't exist in the database anymore.
func (t *Tangle) cleanupChildren() error {

	type child struct {
		blockID      iotago.BlockID
		childBlockID iotago.BlockID
	}

	start := time.Now()

	childrenToDelete := make(map[[iotago.BlockIDLength + iotago.BlockIDLength]byte]*child)

	lastStatusTime := time.Now()
	var childCounter int64
	t.storage.NonCachedStorage().ForEachChild(func(blockID iotago.BlockID, childBlockID iotago.BlockID) bool {
		childCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d children", childCounter)
		}

		childrenMapKey := [iotago.BlockIDLength + iotago.BlockIDLength]byte{}
		copy(childrenMapKey[:iotago.BlockIDLength], blockID[:])
		copy(childrenMapKey[iotago.BlockIDLength:], childBlockID[:])

		// we don't check if the parent still exists, to speed up the revalidation of children by 50%.
		// if children entries would remain, but the block is missing, we would never start a walk from the
		// parent block, since we always walk the future cone.
		/*
			// delete child if block metadata doesn't exist
			if !t.storage.BlockMetadataExistsInStore(blockID) {
				childrenToDelete[childrenMapKey] = &child{blockID: blockID, childBlockID: childBlockID}
			}
		*/

		// delete child if child block metadata doesn't exist
		if !t.storage.BlockMetadataExistsInStore(childBlockID) {
			childrenToDelete[childrenMapKey] = &child{blockID: blockID, childBlockID: childBlockID}
		}

		return true
	})
	t.LogInfof("analyzed %d children", childCounter)

	if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(childrenToDelete)
	var deletionCounter int64
	for _, child := range childrenToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting children ... %d/%d (%0.2f%%). %v left ...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteChild(child.blockID, child.childBlockID)
	}

	t.storage.FlushChildrenStorage()

	t.LogInfof("deleting children ... %d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all unreferenced blocks that are left in the database (we don't need them since we deleted all unreferenced blocks).
func (t *Tangle) cleanupUnreferencedBlocks() error {

	start := time.Now()

	unreferencedMilestoneIndexes := make(map[iotago.MilestoneIndex]struct{})

	lastStatusTime := time.Now()
	var unreferencedBlocksCounter int64
	t.storage.NonCachedStorage().ForEachUnreferencedBlock(func(msIndex iotago.MilestoneIndex, _ iotago.BlockID) bool {
		unreferencedBlocksCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d unreferenced blocks", unreferencedBlocksCounter)
		}

		unreferencedMilestoneIndexes[msIndex] = struct{}{}

		return true
	})
	t.LogInfof("analyzed %d unreferenced blocks", unreferencedBlocksCounter)

	if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(unreferencedMilestoneIndexes)
	var deletionCounter int64
	for msIndex := range unreferencedMilestoneIndexes {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := contextutils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting unreferenced blocks ... %d/%d (%0.2f%%). %v left ...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteUnreferencedBlocks(msIndex)
	}

	t.storage.FlushUnreferencedBlocksStorage()

	t.LogInfof("deleting unreferenced blocks ... %d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// apply the ledger from the last snapshot to the database.
func (t *Tangle) applySnapshotLedger(snapshotInfo *storage.SnapshotInfo, snapshotImporter *snapshot.Importer) error {

	t.LogInfo("applying snapshot balances to the ledger state ...")

	// set the confirmed milestone index to 0.
	// the correct milestone index will be applied during "ImportSnapshots"
	t.syncManager.OverwriteConfirmedMilestoneIndex(0)

	// Restore the ledger state of the last snapshot
	if err := snapshotImporter.ImportSnapshots(t.shutdownCtx); err != nil {
		t.LogPanic(err)
	}

	if err := t.storage.CheckLedgerState(); err != nil {
		t.LogPanic(err)
	}

	ledgerIndex, err := t.storage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		t.LogPanic(err)
	}

	if snapshotInfo.SnapshotIndex() != ledgerIndex {
		return ErrSnapshotIndexWrong
	}

	t.LogInfo("applying snapshot balances to the ledger state ... done!")

	return nil
}
