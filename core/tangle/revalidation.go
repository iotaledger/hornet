package tangle

import (
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
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
//			- Balances of latest solid milestone		=> will be removed and replaced with snapshot milestone
//			- Balances of snapshot milestone			=> should be consistent (total iotas are checked)
//			- Balance diffs of every solid milestone	=> will be removed and added again by confirmation
//
func revalidateDatabase() error {

	// mark the database as tainted forever.
	// this is used to signal the coordinator plugin that it should never use a revalidated database.
	deps.Tangle.MarkDatabaseTainted()

	start := time.Now()

	snapshotInfo := deps.Tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		return ErrSnapshotInfoMissing
	}

	latestMilestoneIndex := deps.Tangle.SearchLatestMilestoneIndexInStore()

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	log.Infof("reverting database state back from %d to local snapshot %d (this might take a while)... ", latestMilestoneIndex, snapshotInfo.SnapshotIndex)

	// delete milestone data newer than the local snapshot
	if err := cleanupMilestones(snapshotInfo); err != nil {
		return err
	}

	// deletes all ledger diffs which have a confirmation milestone newer than the last local snapshot's milestone.
	if err := cleanupLedgerDiffs(snapshotInfo); err != nil {
		return err
	}

	// clean up messages which are above the local snapshot
	if err := cleanupMessages(snapshotInfo); err != nil {
		return err
	}

	// deletes all message metadata where the msg doesn't exist in the database anymore.
	if err := cleanupMessageMetadata(); err != nil {
		return err
	}

	// deletes all children where the msg doesn't exist in the database anymore.
	if err := cleanupChildren(); err != nil {
		return err
	}

	// deletes all addresses where the msg doesn't exist in the database anymore.
	if err := cleanupAddresses(); err != nil {
		return err
	}

	// deletes all unreferenced msgs that are left in the database (we do not need them since we deleted all unreferenced msgs).
	if err := cleanupUnreferencedMsgs(); err != nil {
		return err
	}

	log.Info("flushing storages...")
	deps.Tangle.FlushStorages()
	log.Info("flushing storages... done!")

	// apply the ledger from the last snapshot to the database
	if err := applySnapshotLedger(snapshotInfo); err != nil {
		return err
	}

	log.Infof("reverted state back to local snapshot %d, took %v", snapshotInfo.SnapshotIndex, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func cleanupMilestones(info *tangle.SnapshotInfo) error {

	start := time.Now()

	milestonesToDelete := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var milestonesCounter int64
	deps.Tangle.ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		milestonesCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return false
			}

			log.Infof("analyzed %d milestones", milestonesCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex {
			return true
		}

		milestonesToDelete[msIndex] = struct{}{}

		return true
	}, true)

	if CorePlugin.Daemon().IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(milestonesToDelete)
	var deletionCounter int64
	for msIndex := range milestonesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting milestones...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		deps.Tangle.DeleteUnreferencedMessages(msIndex)
		/*
			if err := deps.Tangle.DeleteLedgerDiffForMilestone(msIndex); err != nil {
				panic(err)
			}
		*/

		deps.Tangle.DeleteMilestone(msIndex)
	}

	deps.Tangle.FlushUnreferencedMessagesStorage()
	deps.Tangle.FlushMilestoneStorage()

	log.Infof("deleting milestones...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all ledger diffs which have a confirmation milestone newer than the last local snapshot's milestone.
func cleanupLedgerDiffs(info *tangle.SnapshotInfo) error {
	return nil
	/*
		start := time.Now()

		ledgerDiffsToDelete := make(map[milestone.Index]struct{})

		lastStatusTime := time.Now()
		var ledgerDiffsCounter int64
		deps.Tangle.ForEachLedgerDiffHash(func(msIndex milestone.Index, address hornet.Hash) bool {
			ledgerDiffsCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if CorePlugin.Daemon().IsStopped() {
					return false
				}

				log.Infof("analyzed %d ledger diffs", ledgerDiffsCounter)
			}

			// do not delete older milestones
			if msIndex <= info.SnapshotIndex {
				return true
			}

			ledgerDiffsToDelete[msIndex] = struct{}{}

			return true
		}, true)

		if CorePlugin.Daemon().IsStopped() {
			return tangle.ErrOperationAborted
		}

		total := len(ledgerDiffsToDelete)
		var deletionCounter int64
		for msIndex := range ledgerDiffsToDelete {
			deletionCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if CorePlugin.Daemon().IsStopped() {
					return tangle.ErrOperationAborted
				}

				percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
				log.Infof("deleting ledger diffs...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
			}

			deps.Tangle.DeleteLedgerDiffForMilestone(msIndex)
		}

		log.Infof("deleting ledger diffs...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

		return nil
	*/
}

// deletes all messages which are not referenced, not solid or
// their confirmation milestone is newer than the last local snapshot's milestone.
func cleanupMessages(info *tangle.SnapshotInfo) error {

	start := time.Now()

	messagesToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var txsCounter int64
	deps.Tangle.ForEachMessageID(func(messageID *hornet.MessageID) bool {
		txsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return false
			}

			log.Infof("analyzed %d messages", txsCounter)
		}

		storedTxMeta := deps.Tangle.GetStoredMetadataOrNil(messageID)

		// delete message if metadata doesn't exist
		if storedTxMeta == nil {
			messagesToDelete[messageID.MapKey()] = struct{}{}
			return true
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			messagesToDelete[messageID.MapKey()] = struct{}{}
			return true
		}

		// not referenced or above snapshot index
		if referenced, by := storedTxMeta.GetReferenced(); !referenced || by > info.SnapshotIndex {
			messagesToDelete[messageID.MapKey()] = struct{}{}
			return true
		}

		return true
	}, true)
	log.Infof("analyzed %d messages", txsCounter)

	if CorePlugin.Daemon().IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(messagesToDelete)
	var deletionCounter int64
	for messageID := range messagesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting messages...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		deps.Tangle.DeleteMessage(hornet.MessageIDFromMapKey(messageID))
	}

	deps.Tangle.FlushMessagesStorage()

	log.Infof("deleting messages...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all message metadata where the msg doesn't exist in the database anymore.
func cleanupMessageMetadata() error {

	start := time.Now()

	metadataToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var metadataCounter int64
	deps.Tangle.ForEachMessageMetadataMessageID(func(messageID *hornet.MessageID) bool {
		metadataCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return false
			}

			log.Infof("analyzed %d message metadata", metadataCounter)
		}

		// delete metadata if message doesn't exist
		if !deps.Tangle.MessageExistsInStore(messageID) {
			metadataToDelete[messageID.MapKey()] = struct{}{}
		}

		return true
	}, true)
	log.Infof("analyzed %d message metadata", metadataCounter)

	if CorePlugin.Daemon().IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(metadataToDelete)
	var deletionCounter int64
	for messageID := range metadataToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting message metadata...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		deps.Tangle.DeleteMessageMetadata(hornet.MessageIDFromMapKey(messageID))
	}

	deps.Tangle.FlushMessagesStorage()

	log.Infof("deleting message metadata...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all children where the msg doesn't exist in the database anymore.
func cleanupChildren() error {

	type child struct {
		messageID      *hornet.MessageID
		childMessageID *hornet.MessageID
	}

	start := time.Now()

	childrenToDelete := make(map[string]*child)

	lastStatusTime := time.Now()
	var childCounter int64
	deps.Tangle.ForEachChild(func(messageID *hornet.MessageID, childMessageID *hornet.MessageID) bool {
		childCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return false
			}

			log.Infof("analyzed %d children", childCounter)
		}

		childrenMapKey := messageID.MapKey() + childMessageID.MapKey()

		// delete child if message doesn't exist
		if !deps.Tangle.MessageExistsInStore(messageID) {
			childrenToDelete[childrenMapKey] = &child{messageID: messageID, childMessageID: childMessageID}
		}

		// delete child if child message doesn't exist
		if !deps.Tangle.MessageExistsInStore(childMessageID) {
			childrenToDelete[childrenMapKey] = &child{messageID: messageID, childMessageID: childMessageID}
		}

		return true
	}, true)
	log.Infof("analyzed %d children", childCounter)

	if CorePlugin.Daemon().IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(childrenToDelete)
	var deletionCounter int64
	for _, child := range childrenToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting children...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		deps.Tangle.DeleteChild(child.messageID, child.childMessageID)
	}

	deps.Tangle.FlushChildrenStorage()

	log.Infof("deleting children...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all addresses where the msg doesn't exist in the database anymore.
func cleanupAddresses() error {
	return nil

	/*
		type address struct {
			address   hornet.Hash
			messageID hornet.Hash
		}

		addressesToDelete := make(map[string]*address)

		start := time.Now()

		lastStatusTime := time.Now()
		var addressesCounter int64
		deps.Tangle.ForEachAddress(func(addressHash hornet.Hash, messageID hornet.Hash, isValue bool) bool {
			addressesCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if CorePlugin.Daemon().IsStopped() {
					return false
				}

				log.Infof("analyzed %d addresses", addressesCounter)
			}

			// delete address if message doesn't exist
			if !deps.Tangle.MessageExistsInStore(messageID) {
				addressesToDelete[messageID.MapKey()] = &address{address: addressHash, messageID: messageID}
			}

			return true
		}, true)
		log.Infof("analyzed %d addresses", addressesCounter)

		if CorePlugin.Daemon().IsStopped() {
			return tangle.ErrOperationAborted
		}

		total := len(addressesToDelete)
		var deletionCounter int64
		for _, addr := range addressesToDelete {
			deletionCounter++

			if time.Since(lastStatusTime) >= printStatusInterval {
				lastStatusTime = time.Now()

				if CorePlugin.Daemon().IsStopped() {
					return tangle.ErrOperationAborted
				}

				percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
				log.Infof("deleting addresses...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
			}

			deps.Tangle.DeleteAddress(addr.address, addr.messageID)
		}

		deps.Tangle.FlushAddressStorage()

		log.Infof("deleting addresses...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

		return nil
	*/
}

// deletes all unreferenced msgs that are left in the database (we do not need them since we deleted all unreferenced msgs).
func cleanupUnreferencedMsgs() error {

	start := time.Now()

	unreferencedMilestoneIndexes := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var unreferencedTxsCounter int64
	deps.Tangle.ForEachUnreferencedMessage(func(msIndex milestone.Index, messageID *hornet.MessageID) bool {
		unreferencedTxsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return false
			}

			log.Infof("analyzed %d unreferenced msgs", unreferencedTxsCounter)
		}

		unreferencedMilestoneIndexes[msIndex] = struct{}{}

		return true
	}, true)
	log.Infof("analyzed %d unreferenced msgs", unreferencedTxsCounter)

	if CorePlugin.Daemon().IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(unreferencedMilestoneIndexes)
	var deletionCounter int64
	for msIndex := range unreferencedMilestoneIndexes {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if CorePlugin.Daemon().IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting unreferenced msgs...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		deps.Tangle.DeleteUnreferencedMessages(msIndex)
	}

	deps.Tangle.FlushUnreferencedMessagesStorage()

	log.Infof("deleting unreferenced msgs...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// apply the ledger from the last snapshot to the database
func applySnapshotLedger(info *tangle.SnapshotInfo) error {

	log.Info("applying snapshot balances to the ledger state...")

	/*
		// Get the ledger state of the last snapshot
		snapshotBalances, snapshotIndex, err := deps.Tangle.GetAllSnapshotBalances(nil)
		if err != nil {
			return err
		}

		if info.SnapshotIndex != snapshotIndex {
			return ErrSnapshotIndexWrong
		}

		// Store the snapshot balances as the current valid ledger
		if err = deps.Tangle.StoreLedgerBalancesInDatabase(snapshotBalances, snapshotIndex); err != nil {
			return err
		}
		log.Info("applying snapshot balances to the ledger state ... done!")
	*/
	// Set the valid solid milestone index
	deps.Tangle.OverwriteSolidMilestoneIndex(info.SnapshotIndex)

	return nil
}
