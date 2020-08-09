package tangle

import (
	"errors"
	"time"

	"github.com/iotaledger/hive.go/daemon"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
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
// This function tries to restore a clean database state by deleting all existing transactions
// since last local snapshot, deleting all ledger states and changes, loading valid snapshot ledger state.
//
// This way HORNET should be able to re-solidify the existing tangle in the database.
//
// Object Storages:
// 		Stored with caching:
//			- TxRaw (synced)					=> will be removed and added again by requesting the tx at solidification
//			- TxMetadata (synced)				=> will be removed and added again if missing by receiving the tx (if not => reset)
//			- BundleTransaction (synced)		=> will be removed and added again if missing by receiving the tx
//			- Bundle (always)					=> will be removed and added again if missing by receiving the tx
//			- Approver (synced)					=> will be removed and added again if missing by receiving the tx
//
// 		Stored without caching:
//			- Tag								=> will be removed and added again if missing by receiving the tx
//			- Address							=> will be removed and added again if missing by receiving the tx
//			- UnconfirmedTx 					=> will be removed at pruning anyway
//			- Milestone							=> will be removed and added again by receiving the tx
//			- SpentAddresses					=> will be removed and added again if missing by receiving the tx
//
// Database:
// 		- LedgerState
//			- Balances of latest solid milestone		=> will be removed and replaced with snapshot milestone
//			- Balances of snapshot milestone			=> should be consistent (total iotas are checked)
//			- Balance diffs of every solid milestone	=> will be removed and added again by confirmation
//
func revalidateDatabase() error {

	start := time.Now()

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		return ErrSnapshotInfoMissing
	}

	latestMilestoneIndex := tangle.SearchLatestMilestoneIndexInStore()

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	log.Infof("reverting database state back to local snapshot %d...", snapshotInfo.SnapshotIndex)

	// delete milestone data newer than the local snapshot
	if err := cleanupMilestones(snapshotInfo); err != nil {
		return err
	}

	// clean up transactions which are above the local snapshot
	if err := cleanupTransactions(snapshotInfo); err != nil {
		return err
	}

	// deletes all transaction metadata where the tx doesn't exist in the database anymore.
	if err := cleanupTransactionMetadata(); err != nil {
		return err
	}

	// deletes all bundles where a single tx of the bundle doesn't exist in the database anymore.
	if err := cleanupBundles(); err != nil {
		return err
	}

	// deletes all bundles transactions where the tx doesn't exist in the database anymore.
	if err := cleanupBundleTransactions(); err != nil {
		return err
	}

	// deletes all approvers where the tx doesn't exist in the database anymore.
	if err := cleanupApprovers(); err != nil {
		return err
	}

	// deletes all tags where the tx doesn't exist in the database anymore.
	if err := cleanupTags(); err != nil {
		return err
	}

	// deletes all addresses where the tx doesn't exist in the database anymore.
	if err := cleanupAddresses(); err != nil {
		return err
	}

	tangle.FlushStorages()

	// Get the ledger state of the last snapshot
	snapshotBalances, snapshotIndex, err := tangle.GetAllSnapshotBalances(nil)
	if err != nil {
		return err
	}

	if snapshotInfo.SnapshotIndex != snapshotIndex {
		return ErrSnapshotIndexWrong
	}

	// Store the snapshot balances as the current valid ledger
	if err = tangle.StoreLedgerBalancesInDatabase(snapshotBalances, snapshotIndex); err != nil {
		return err
	}

	// Set the valid solid milestone index
	tangle.OverwriteSolidMilestoneIndex(snapshotInfo.SnapshotIndex)

	log.Infof("reverted state back to local snapshot %d, took %v", snapshotInfo.SnapshotIndex, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func cleanupMilestones(info *tangle.SnapshotInfo) error {

	start := time.Now()

	var milestonesCounter int64
	var deletionCounter int64
	tangle.ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		milestonesCounter++

		if (milestonesCounter % 1000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d milestones", milestonesCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex {
			return true
		}

		tangle.DeleteUnconfirmedTxs(msIndex)
		if err := tangle.DeleteLedgerDiffForMilestone(msIndex); err != nil {
			panic(err)
		}

		tangle.DeleteMilestone(msIndex)
		deletionCounter++

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d milestones deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all transactions which are not confirmed, not solid or
// their confirmation milestone is newer than the last local snapshot's milestone.
func cleanupTransactions(info *tangle.SnapshotInfo) error {

	start := time.Now()

	var txsCounter int64
	var deletionCounter int64
	tangle.ForEachTransactionHash(func(txHash hornet.Hash) bool {
		txsCounter++

		if (txsCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d transactions", txsCounter)
		}

		storedTxMeta := tangle.GetStoredMetadataOrNil(txHash)

		// delete transaction if metadata doesn't exist
		if storedTxMeta == nil {
			tangle.DeleteTransaction(txHash)
			deletionCounter++
			return true
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			tangle.DeleteTransaction(txHash)
			deletionCounter++
			return true
		}

		// not confirmed or above snapshot index
		if confirmed, by := storedTxMeta.GetConfirmed(); !confirmed || by > info.SnapshotIndex {
			tangle.DeleteTransaction(txHash)
			deletionCounter++
			return true
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d transactions deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all transaction metadata where the tx doesn't exist in the database anymore.
func cleanupTransactionMetadata() error {

	start := time.Now()

	var metadataCounter int64
	var deletionCounter int64
	tangle.ForEachTransactionMetadataHash(func(txHash hornet.Hash) bool {
		metadataCounter++

		if (metadataCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d transaction metadata", metadataCounter)
		}

		// delete metadata if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			tangle.DeleteTransactionMetadata(txHash)
			deletionCounter++
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d transaction metadata deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all bundles where a single tx of the bundle doesn't exist in the database anymore.
func cleanupBundles() error {

	start := time.Now()

	var bundleCounter int64
	var deletionCounter int64
	tangle.ForEachBundleHash(func(tailTxHash hornet.Hash) bool {
		bundleCounter++

		if (bundleCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d bundles", bundleCounter)
		}

		bundle := tangle.GetStoredBundleOrNil(tailTxHash)
		if bundle == nil {
			return true
		}

		for _, txHash := range bundle.GetTxHashes() {
			// delete bundle if a single transaction of the bundle doesn't exist
			if !tangle.TransactionExistsInStore(txHash) {
				tangle.DeleteBundle(tailTxHash)
				deletionCounter++
				return true
			}
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d bundles deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all bundles transactions where the tx doesn't exist in the database anymore.
func cleanupBundleTransactions() error {

	start := time.Now()

	var bundleTxsCounter int64
	var deletionCounter int64
	tangle.ForEachBundleTransaction(func(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) bool {
		bundleTxsCounter++

		if (bundleTxsCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d bundle transactions", bundleTxsCounter)
		}

		// delete bundle transaction if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			tangle.DeleteBundleTransaction(bundleHash, txHash, isTail)
			deletionCounter++
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d bundle transactions deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all approvers where the tx doesn't exist in the database anymore.
func cleanupApprovers() error {

	start := time.Now()

	var approverCounter int64
	var deletionCounter int64
	tangle.ForEachApprover(func(txHash hornet.Hash, approverHash hornet.Hash) bool {
		approverCounter++

		if (approverCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d approvers", approverCounter)
		}

		// delete approver if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			tangle.DeleteApprover(txHash, approverHash)
			deletionCounter++
		}

		// delete approver if approver transaction doesn't exist
		if !tangle.TransactionExistsInStore(approverHash) {
			tangle.DeleteApprover(txHash, approverHash)
			deletionCounter++
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d approvers deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all tags where the tx doesn't exist in the database anymore.
func cleanupTags() error {

	start := time.Now()

	var tagsCounter int64
	var deletionCounter int64
	tangle.ForEachTag(func(txTag hornet.Hash, txHash hornet.Hash) bool {
		tagsCounter++

		if (tagsCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d tags", tagsCounter)
		}

		// delete tag if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			tangle.DeleteTag(txTag, txHash)
			deletionCounter++
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d tags deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all addresses where the tx doesn't exist in the database anymore.
func cleanupAddresses() error {

	start := time.Now()

	var addressesCounter int64
	var deletionCounter int64
	tangle.ForEachAddress(func(address hornet.Hash, txHash hornet.Hash, isValue bool) bool {
		addressesCounter++

		if (addressesCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d addresses", addressesCounter)
		}

		// delete address if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			tangle.DeleteAddress(address, txHash)
			deletionCounter++
		}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d addresses deleted, took %v", deletionCounter, time.Since(start).Truncate(time.Millisecond))

	return nil
}
