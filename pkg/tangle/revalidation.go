package tangle

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	// printStatusInterval is the interval for printing status messages
	printStatusInterval = 2 * time.Second
)

var (
	ErrSnapshotInfoMissing                   = errors.New("snapshot information not found in database")
	ErrLatestMilestoneOlderThanSnapshotIndex = errors.New("latest milestone in the database is older than the snapshot index")
	ErrSnapshotIndexWrong                    = errors.New("snapshot index does not fit the snapshot ledger index")
)

// RevalidateDatabase tries to revalidate a corrupted database (after an unclean node shutdown/crash)
//
// HORNET uses caches for almost all tangle related data.
// If the node crashes, it is not guaranteed that all data in the cache was already persisted to the disk.
// Thats why we flag the database as corrupted.
//
// This function tries to restore a clean database state by deleting all existing messages
// since last local snapshot, deleting all ledger states and changes, and loading a valid snapshot ledger state.
//
// This way HORNET should be able to re-solidify the existing tangle in the database.
//
// Object Storages:
//		- Milestone							=> will be removed and added again if missing by receiving the msg
//		- Message							=> will be removed and added again by requesting the msg at solidification
//		- MessageMetadata   				=> will be removed and added again if missing by receiving the msg
//		- Children							=> will be removed and added again if missing by receiving the msg
//		- Indexation						=> will be removed and added again if missing by receiving the msg
//		- UnreferencedMessage 				=> will be removed at pruning anyway
//
// Database:
// 		- LedgerState
//			- Unspent						=> will be removed and loaded again from last snapshot
//			- Spent							=> will be removed and loaded again from last snapshot
//			- Balances						=> will be removed and loaded again from last snapshot
//			- Diffs							=> will be removed and loaded again from last snapshot
//			- Treasury						=> will be removed and loaded again from last snapshot
//			- Receipts						=> will be removed and loaded again from last snapshot (if pruneReceipts is enabled)
func (t *Tangle) RevalidateDatabase(snapshotManager *snapshot.SnapshotManager, pruneReceipts bool) error {

	// mark the database as tainted forever.
	// this is used to signal the coordinator plugin that it should never use a revalidated database.
	if err := t.storage.MarkDatabasesTainted(); err != nil {
		t.LogPanic(err)
	}

	start := time.Now()

	snapshotInfo := t.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return ErrSnapshotInfoMissing
	}

	latestMilestoneIndex := t.storage.SearchLatestMilestoneIndexInStore()

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	// check if the ledger index of the snapshot files fit the revalidation target.
	snapshotLedgerIndex, err := snapshotManager.SnapshotsFilesLedgerIndex()
	if err != nil {
		return err
	}

	if snapshotLedgerIndex != snapshotInfo.SnapshotIndex {
		return fmt.Errorf("snapshot files (index: %d) do not fit the revalidation target (index: %d)", snapshotLedgerIndex, snapshotInfo.SnapshotIndex)
	}

	t.LogInfof("reverting database state back from %d to snapshot %d (this might take a while)... ", latestMilestoneIndex, snapshotInfo.SnapshotIndex)

	// deletes all ledger entries (unspent, spent, diffs, balances, treasury, receipts).
	if err := t.cleanupLedger(pruneReceipts); err != nil {
		return err
	}

	// delete milestone data newer than the local snapshot
	if err := t.cleanupMilestones(snapshotInfo); err != nil {
		return err
	}

	// deletes all messages which metadata doesn't exist anymore, are not referenced, not solid or
	// their confirmation milestone is newer than the last local snapshot's milestone.
	if err := t.cleanupMessages(snapshotInfo); err != nil {
		return err
	}

	// deletes all message metadata where the messages doesn't exist in the database anymore.
	if err := t.cleanupMessageMetadata(); err != nil {
		return err
	}

	// deletes all children where the message metadata doesn't exist in the database anymore.
	if err := t.cleanupChildren(); err != nil {
		return err
	}

	// deletes all unreferenced messages that are left in the database (we do not need them since we deleted all unreferenced messages).
	if err := t.cleanupUnreferencedMsgs(); err != nil {
		return err
	}

	t.LogInfo("flushing storages...")
	t.storage.FlushStorages()
	t.LogInfo("flushing storages... done!")

	// apply the ledger from the last snapshot to the database
	if err := t.applySnapshotLedger(snapshotInfo, snapshotManager); err != nil {
		return err
	}

	t.LogInfof("reverted state back to snapshot %d, took %v", snapshotInfo.SnapshotIndex, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all ledger entries (unspent, spent, diffs, balances, treasury, receipts).
func (t *Tangle) cleanupLedger(pruneReceipts bool) error {

	start := time.Now()

	t.LogInfo("clearing ledger... ")
	if err := t.storage.UTXOManager().ClearLedger(pruneReceipts); err != nil {
		return err
	}
	t.LogInfof("clearing ledger... done. took %v", time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func (t *Tangle) cleanupMilestones(info *storage.SnapshotInfo) error {

	start := time.Now()

	milestonesToDelete := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var milestonesCounter int64
	t.storage.NonCachedStorage().ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		milestonesCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d milestones", milestonesCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex {
			return true
		}

		milestonesToDelete[msIndex] = struct{}{}

		return true
	})

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(milestonesToDelete)
	var deletionCounter int64
	for msIndex := range milestonesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting milestones...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteUnreferencedMessages(msIndex)
		t.storage.DeleteMilestone(msIndex)
	}

	t.storage.FlushUnreferencedMessagesStorage()
	t.storage.FlushMilestoneStorage()

	t.LogInfof("deleting milestones...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all messages which metadata doesn't exist anymore, are not referenced, not solid or
// their confirmation milestone is newer than the last local snapshot's milestone.
func (t *Tangle) cleanupMessages(info *storage.SnapshotInfo) error {

	start := time.Now()

	messagesToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var txsCounter int64
	t.storage.NonCachedStorage().ForEachMessageID(func(messageID hornet.MessageID) bool {
		txsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d messages", txsCounter)
		}

		storedTxMeta := t.storage.StoredMetadataOrNil(messageID)

		// delete message if metadata doesn't exist
		if storedTxMeta == nil {
			messagesToDelete[messageID.ToMapKey()] = struct{}{}
			return true
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			messagesToDelete[messageID.ToMapKey()] = struct{}{}
			return true
		}

		// not referenced or above snapshot index
		if referenced, by := storedTxMeta.ReferencedWithIndex(); !referenced || by > info.SnapshotIndex {
			messagesToDelete[messageID.ToMapKey()] = struct{}{}
			return true
		}

		return true
	})
	t.LogInfof("analyzed %d messages", txsCounter)

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(messagesToDelete)
	var deletionCounter int64
	for messageID := range messagesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting messages...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteMessage(hornet.MessageIDFromMapKey(messageID))
	}

	t.storage.FlushMessagesStorage()

	t.LogInfof("deleting messages...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all message metadata where the message doesn't exist in the database anymore.
func (t *Tangle) cleanupMessageMetadata() error {

	start := time.Now()

	metadataToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var metadataCounter int64
	t.storage.NonCachedStorage().ForEachMessageMetadataMessageID(func(messageID hornet.MessageID) bool {
		metadataCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d message metadata", metadataCounter)
		}

		// delete metadata if message doesn't exist
		if !t.storage.MessageExistsInStore(messageID) {
			metadataToDelete[messageID.ToMapKey()] = struct{}{}
		}

		return true
	})
	t.LogInfof("analyzed %d message metadata", metadataCounter)

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(metadataToDelete)
	var deletionCounter int64
	for messageID := range metadataToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting message metadata...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteMessageMetadata(hornet.MessageIDFromMapKey(messageID))
	}

	t.storage.FlushMessagesStorage()

	t.LogInfof("deleting message metadata...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all children where the message metadata doesn't exist in the database anymore.
func (t *Tangle) cleanupChildren() error {

	type child struct {
		messageID      hornet.MessageID
		childMessageID hornet.MessageID
	}

	start := time.Now()

	childrenToDelete := make(map[string]*child)

	lastStatusTime := time.Now()
	var childCounter int64
	t.storage.NonCachedStorage().ForEachChild(func(messageID hornet.MessageID, childMessageID hornet.MessageID) bool {
		childCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d children", childCounter)
		}

		childrenMapKey := messageID.ToMapKey() + childMessageID.ToMapKey()

		// we do not check if the parent still exists, to speed up the revalidation of children by 50%.
		// if children entries would remain, but the message is missing, we would never start a walk from the
		// parent message, since we always walk the future cone.
		/*
			// delete child if message metadata doesn't exist
			if !t.storage.MessageMetadataExistsInStore(messageID) {
				childrenToDelete[childrenMapKey] = &child{messageID: messageID, childMessageID: childMessageID}
			}
		*/

		// delete child if child message metadata doesn't exist
		if !t.storage.MessageMetadataExistsInStore(childMessageID) {
			childrenToDelete[childrenMapKey] = &child{messageID: messageID, childMessageID: childMessageID}
		}

		return true
	})
	t.LogInfof("analyzed %d children", childCounter)

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(childrenToDelete)
	var deletionCounter int64
	for _, child := range childrenToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting children...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteChild(child.messageID, child.childMessageID)
	}

	t.storage.FlushChildrenStorage()

	t.LogInfof("deleting children...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all unreferenced messages that are left in the database (we do not need them since we deleted all unreferenced messages).
func (t *Tangle) cleanupUnreferencedMsgs() error {

	start := time.Now()

	unreferencedMilestoneIndexes := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var unreferencedTxsCounter int64
	t.storage.NonCachedStorage().ForEachUnreferencedMessage(func(msIndex milestone.Index, _ hornet.MessageID) bool {
		unreferencedTxsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.LogInfof("analyzed %d unreferenced messages", unreferencedTxsCounter)
		}

		unreferencedMilestoneIndexes[msIndex] = struct{}{}

		return true
	})
	t.LogInfof("analyzed %d unreferenced messages", unreferencedTxsCounter)

	if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
		return err
	}

	total := len(unreferencedMilestoneIndexes)
	var deletionCounter int64
	for msIndex := range unreferencedMilestoneIndexes {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return err
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			t.LogInfof("deleting unreferenced messages...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteUnreferencedMessages(msIndex)
	}

	t.storage.FlushUnreferencedMessagesStorage()

	t.LogInfof("deleting unreferenced messages...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// apply the ledger from the last snapshot to the database
func (t *Tangle) applySnapshotLedger(snapshotInfo *storage.SnapshotInfo, snapshotManager *snapshot.SnapshotManager) error {

	t.LogInfo("applying snapshot balances to the ledger state...")

	// set the confirmed milestone index to 0.
	// the correct milestone index will be applied during "ImportSnapshots"
	t.syncManager.OverwriteConfirmedMilestoneIndex(0)

	// Restore the ledger state of the last snapshot
	if err := snapshotManager.ImportSnapshots(t.shutdownCtx); err != nil {
		t.LogPanic(err)
	}

	if err := snapshotManager.CheckCurrentSnapshot(snapshotInfo); err != nil {
		t.LogPanic(err)
	}

	ledgerIndex, err := t.storage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		t.LogPanic(err)
	}

	if snapshotInfo.SnapshotIndex != ledgerIndex {
		return ErrSnapshotIndexWrong
	}

	t.LogInfo("applying snapshot balances to the ledger state ... done!")

	return nil
}
