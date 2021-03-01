package tangle

import (
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
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

// revalidateDatabase tries to revalidate a corrupted database (after an unclean node shutdown/crash)
//
// HORNET uses caches for almost all tangle related data.
// If the node crashes, it is not guaranteed that all data in the cache was already persisted to the disk.
// Thats why we flag the database as corrupted.
//
// This function tries to restore a clean database state by deleting all existing messages
// since last local snapshot, deleting all ledger states and changes, loading valid snapshot ledger state.
//
// This way HORNET should be able to re-solidify the existing tangle in the database.
//
// Object Storages:
// 		Stored with caching:
//			- TxRaw (synced)					=> will be removed and added again by requesting the msg at solidification
//			- TxMetadata (synced)				=> will be removed and added again if missing by receiving the msg (if not => reset)
//			- Message (always)					=> will be removed and added again if missing by receiving the msg
//			- Child (synced)					=> will be removed and added again if missing by receiving the msg
//
// 		Stored without caching:
//			- Tag								=> will be removed and added again if missing by receiving the msg
//			- Address							=> will be removed and added again if missing by receiving the msg
//			- UnreferencedMessage 				=> will be removed at pruning anyway
//			- Milestone							=> will be removed and added again by receiving the msg
//
// Database:
// 		- LedgerState
//			- Balances of confirmed milestone				=> will be removed and replaced with snapshot milestone
//			- Balances of snapshot milestone				=> should be consistent (total iotas are checked)
//			- Balance diffs of every confirmed milestone	=> will be removed and added again by confirmation
//
func (t *Tangle) RevalidateDatabase() error {

	// mark the database as tainted forever.
	// this is used to signal the coordinator plugin that it should never use a revalidated database.
	t.storage.MarkDatabaseTainted()

	start := time.Now()

	snapshotInfo := t.storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		return ErrSnapshotInfoMissing
	}

	latestMilestoneIndex := t.storage.SearchLatestMilestoneIndexInStore()

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	t.log.Infof("reverting database state back from %d to snapshot %d (this might take a while)... ", latestMilestoneIndex, snapshotInfo.SnapshotIndex)

	// delete milestone data newer than the local snapshot
	if err := t.cleanupMilestones(snapshotInfo); err != nil {
		return err
	}

	// deletes all ledger diffs which have a confirmation milestone newer than the last local snapshot's milestone.
	if err := t.cleanupLedgerDiffs(snapshotInfo); err != nil {
		return err
	}

	// clean up messages which are above the local snapshot
	if err := t.cleanupMessages(snapshotInfo); err != nil {
		return err
	}

	// deletes all message metadata where the msg doesn't exist in the database anymore.
	if err := t.cleanupMessageMetadata(); err != nil {
		return err
	}

	// deletes all children where the msg doesn't exist in the database anymore.
	if err := t.cleanupChildren(); err != nil {
		return err
	}

	// deletes all addresses where the msg doesn't exist in the database anymore.
	if err := t.cleanupAddresses(); err != nil {
		return err
	}

	// deletes all unreferenced msgs that are left in the database (we do not need them since we deleted all unreferenced msgs).
	if err := t.cleanupUnreferencedMsgs(); err != nil {
		return err
	}

	t.log.Info("flushing storages...")
	t.storage.FlushStorages()
	t.log.Info("flushing storages... done!")

	// apply the ledger from the last snapshot to the database
	if err := t.applySnapshotLedger(snapshotInfo); err != nil {
		return err
	}

	t.log.Infof("reverted state back to snapshot %d, took %v", snapshotInfo.SnapshotIndex, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func (t *Tangle) cleanupMilestones(info *storage.SnapshotInfo) error {

	start := time.Now()

	milestonesToDelete := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var milestonesCounter int64
	t.storage.ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		milestonesCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.log.Infof("analyzed %d milestones", milestonesCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex {
			return true
		}

		milestonesToDelete[msIndex] = struct{}{}

		return true
	}, true)

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
			t.log.Infof("deleting milestones...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteUnreferencedMessages(msIndex)
		/*
			if err := t.storage.DeleteLedgerDiffForMilestone(msIndex); err != nil {
				panic(err)
			}
		*/

		t.storage.DeleteMilestone(msIndex)
	}

	t.storage.FlushUnreferencedMessagesStorage()
	t.storage.FlushMilestoneStorage()

	t.log.Infof("deleting milestones...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all ledger diffs which have a confirmation milestone newer than the last local snapshot's milestone.
func (t *Tangle) cleanupLedgerDiffs(info *storage.SnapshotInfo) error {
	return nil
	/*
		start := time.Now()

		ledgerDiffsToDelete := make(map[milestone.Index]struct{})

		lastStatusTime := time.Now()
		var ledgerDiffsCounter int64
		t.storage.ForEachLedgerDiffHash(func(msIndex milestone.Index, address hornet.Hash) bool {
			ledgerDiffsCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
					return false
				}

				t.log.Infof("analyzed %d ledger diffs", ledgerDiffsCounter)
			}

			// do not delete older milestones
			if msIndex <= info.SnapshotIndex {
				return true
			}

			ledgerDiffsToDelete[msIndex] = struct{}{}

			return true
		}, true)

		if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
			return err
		}

		total := len(ledgerDiffsToDelete)
		var deletionCounter int64
		for msIndex := range ledgerDiffsToDelete {
			deletionCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
					return err
				}

				percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
				t.log.Infof("deleting ledger diffs...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
			}

			t.storage.DeleteLedgerDiffForMilestone(msIndex)
		}

		t.log.Infof("deleting ledger diffs...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

		return nil
	*/
}

// deletes all messages which are not referenced, not solid or
// their confirmation milestone is newer than the last local snapshot's milestone.
func (t *Tangle) cleanupMessages(info *storage.SnapshotInfo) error {

	start := time.Now()

	messagesToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var txsCounter int64
	t.storage.ForEachMessageID(func(messageID hornet.MessageID) bool {
		txsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.log.Infof("analyzed %d messages", txsCounter)
		}

		storedTxMeta := t.storage.GetStoredMetadataOrNil(messageID)

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
		if referenced, by := storedTxMeta.GetReferenced(); !referenced || by > info.SnapshotIndex {
			messagesToDelete[messageID.ToMapKey()] = struct{}{}
			return true
		}

		return true
	}, true)
	t.log.Infof("analyzed %d messages", txsCounter)

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
			t.log.Infof("deleting messages...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteMessage(hornet.MessageIDFromMapKey(messageID))
	}

	t.storage.FlushMessagesStorage()

	t.log.Infof("deleting messages...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all message metadata where the msg doesn't exist in the database anymore.
func (t *Tangle) cleanupMessageMetadata() error {

	start := time.Now()

	metadataToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var metadataCounter int64
	t.storage.ForEachMessageMetadataMessageID(func(messageID hornet.MessageID) bool {
		metadataCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.log.Infof("analyzed %d message metadata", metadataCounter)
		}

		// delete metadata if message doesn't exist
		if !t.storage.MessageExistsInStore(messageID) {
			metadataToDelete[messageID.ToMapKey()] = struct{}{}
		}

		return true
	}, true)
	t.log.Infof("analyzed %d message metadata", metadataCounter)

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
			t.log.Infof("deleting message metadata...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteMessageMetadata(hornet.MessageIDFromMapKey(messageID))
	}

	t.storage.FlushMessagesStorage()

	t.log.Infof("deleting message metadata...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all children where the msg doesn't exist in the database anymore.
func (t *Tangle) cleanupChildren() error {

	type child struct {
		messageID      hornet.MessageID
		childMessageID hornet.MessageID
	}

	start := time.Now()

	childrenToDelete := make(map[string]*child)

	lastStatusTime := time.Now()
	var childCounter int64
	t.storage.ForEachChild(func(messageID hornet.MessageID, childMessageID hornet.MessageID) bool {
		childCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.log.Infof("analyzed %d children", childCounter)
		}

		childrenMapKey := messageID.ToMapKey() + childMessageID.ToMapKey()

		// delete child if message doesn't exist
		if !t.storage.MessageExistsInStore(messageID) {
			childrenToDelete[childrenMapKey] = &child{messageID: messageID, childMessageID: childMessageID}
		}

		// delete child if child message doesn't exist
		if !t.storage.MessageExistsInStore(childMessageID) {
			childrenToDelete[childrenMapKey] = &child{messageID: messageID, childMessageID: childMessageID}
		}

		return true
	}, true)
	t.log.Infof("analyzed %d children", childCounter)

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
			t.log.Infof("deleting children...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteChild(child.messageID, child.childMessageID)
	}

	t.storage.FlushChildrenStorage()

	t.log.Infof("deleting children...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all addresses where the msg doesn't exist in the database anymore.
func (t *Tangle) cleanupAddresses() error {
	return nil

	/*
		TODO?

		type address struct {
			address   hornet.Hash
			messageID hornet.Hash
		}

		addressesToDelete := make(map[string]*address)

		start := time.Now()

		lastStatusTime := time.Now()
		var addressesCounter int64
		t.storage.ForEachAddress(func(addressHash hornet.Hash, messageID hornet.Hash, isValue bool) bool {
			addressesCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
					return false
				}

				t.log.Infof("analyzed %d addresses", addressesCounter)
			}

			// delete address if message doesn't exist
			if !t.storage.MessageExistsInStore(messageID) {
				addressesToDelete[messageID.MapKey()] = &address{address: addressHash, messageID: messageID}
			}

			return true
		}, true)
		t.log.Infof("analyzed %d addresses", addressesCounter)

		if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
			return err
		}

		total := len(addressesToDelete)
		var deletionCounter int64
		for _, addr := range addressesToDelete {
			deletionCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
					return err
				}

				percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
				t.log.Infof("deleting addresses...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
			}

			t.storage.DeleteAddress(addr.address, addr.messageID)
		}

		t.storage.FlushAddressStorage()

		t.log.Infof("deleting addresses...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

		return nil
	*/
}

// deletes all unreferenced msgs that are left in the database (we do not need them since we deleted all unreferenced msgs).
func (t *Tangle) cleanupUnreferencedMsgs() error {

	start := time.Now()

	unreferencedMilestoneIndexes := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var unreferencedTxsCounter int64
	t.storage.ForEachUnreferencedMessage(func(msIndex milestone.Index, messageID hornet.MessageID) bool {
		unreferencedTxsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if err := utils.ReturnErrIfCtxDone(t.shutdownCtx, common.ErrOperationAborted); err != nil {
				return false
			}

			t.log.Infof("analyzed %d unreferenced msgs", unreferencedTxsCounter)
		}

		unreferencedMilestoneIndexes[msIndex] = struct{}{}

		return true
	}, true)
	t.log.Infof("analyzed %d unreferenced msgs", unreferencedTxsCounter)

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
			t.log.Infof("deleting unreferenced msgs...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		t.storage.DeleteUnreferencedMessages(msIndex)
	}

	t.storage.FlushUnreferencedMessagesStorage()

	t.log.Infof("deleting unreferenced msgs...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// apply the ledger from the last snapshot to the database
func (t *Tangle) applySnapshotLedger(info *storage.SnapshotInfo) error {

	t.log.Info("applying snapshot balances to the ledger state...")

	/*
		TODO?
		// Get the ledger state of the last snapshot
		snapshotBalances, snapshotIndex, err := t.storage.GetAllSnapshotBalances(nil)
		if err != nil {
			return err
		}

		if info.SnapshotIndex != snapshotIndex {
			return ErrSnapshotIndexWrong
		}

		// Store the snapshot balances as the current valid ledger
		if err = t.storage.StoreLedgerBalancesInDatabase(snapshotBalances, snapshotIndex); err != nil {
			return err
		}
		t.log.Info("applying snapshot balances to the ledger state ... done!")
	*/
	// Set the valid confirmed milestone index
	t.storage.OverwriteConfirmedMilestoneIndex(info.SnapshotIndex)

	return nil
}
