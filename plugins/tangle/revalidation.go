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
	if err := cleanMilestones(snapshotInfo); err != nil {
		return err
	}

	// clean up transactions which are above the local snapshot
	if err := cleanupTransactions(snapshotInfo); err != nil {
		return err
	}

	// clean up bundles of which their tail tx doesn't exist in the database anymore
	if err := cleanupBundles(); err != nil {
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

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func cleanMilestones(info *tangle.SnapshotInfo) error {
	milestonesToDelete := map[milestone.Index]struct{}{}

	tangle.ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		if msIndex > info.SnapshotIndex {
			milestonesToDelete[msIndex] = struct{}{}
		}
		return true
	})

	for msIndex := range milestonesToDelete {
		if daemon.IsStopped() {
			return tangle.ErrOperationAborted
		}

		tangle.DeleteUnconfirmedTxs(msIndex)
		if err := tangle.DeleteLedgerDiffForMilestone(msIndex); err != nil {
			panic(err)
		}

		milestone := tangle.GetCachedMilestoneOrNil(msIndex) // milestone +1
		if milestone == nil {
			return ErrMilestoneNotFound
		}
		msHash := milestone.GetMilestone().Hash
		milestone.Release(true) // milestone -1

		// delete the bundle of the milestone, so the milestone will be created again during syncing
		tangle.DeleteBundle(msHash)
		tangle.DeleteMilestone(msIndex)
	}

	return nil
}

// deletes all transactions (and their bundle, first seen tx, etc.) which are not confirmed,
// not solid, their confirmation milestone is newer/ of which their solidification time is younger
// than the last local snapshot's milestone.
func cleanupTransactions(info *tangle.SnapshotInfo) error {
	txsToDelete := map[string]struct{}{}

	start := time.Now()
	var txCounter int64
	tangle.ForEachTransactionHash(func(txHash hornet.Hash) bool {
		txCounter++

		if (txCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d transactions", txCounter)
		}

		storedTxMeta := tangle.GetStoredMetadataOrNil(txHash)

		// delete transaction if no metadata
		if storedTxMeta == nil {
			txsToDelete[string(txHash)] = struct{}{}
			return true
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			txsToDelete[string(txHash)] = struct{}{}
			return true
		}

		if confirmed, by := storedTxMeta.GetConfirmed(); !confirmed || by > info.SnapshotIndex {
			txsToDelete[string(txHash)] = struct{}{}
		}

		return true
	})

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	var deletionCounter float64
	total := float64(len(txsToDelete))
	lastPercentage := 0
	for txHashToDelete := range txsToDelete {
		if daemon.IsStopped() {
			return tangle.ErrOperationAborted
		}

		percentage := int((deletionCounter / total) * 100)
		if lastPercentage+5 <= percentage {
			lastPercentage = percentage
			log.Infof("reverting (this might take a while)...%d/%d (%d%%)", int(deletionCounter), len(txsToDelete), percentage)
		}

		storedTx := tangle.GetStoredTransactionOrNil(hornet.Hash(txHashToDelete))
		if storedTx == nil {
			continue
		}

		deletionCounter++

		// No need to safely remove the transactions from the bundle,
		// since reattachment txs confirmed by another milestone wouldn't be
		// pruned anyway if they are confirmed before snapshot index.
		tangle.DeleteBundleTransaction(storedTx.GetBundleHash(), storedTx.GetTxHash(), true)
		tangle.DeleteBundleTransaction(storedTx.GetBundleHash(), storedTx.GetTxHash(), false)
		tangle.DeleteBundle(storedTx.GetTxHash())

		// Delete the reference in the approvees
		tangle.DeleteApprover(storedTx.GetTrunkHash(), storedTx.GetTxHash())
		tangle.DeleteApprover(storedTx.GetBranchHash(), storedTx.GetTxHash())

		tangle.DeleteTag(storedTx.GetTag(), storedTx.GetTxHash())
		tangle.DeleteAddress(storedTx.GetAddress(), storedTx.GetTxHash())
		tangle.DeleteApprovers(storedTx.GetTxHash())
		tangle.DeleteTransaction(storedTx.GetTxHash())
	}

	log.Infof("reverted state back to local snapshot %d, %d transactions deleted, took %v", info.SnapshotIndex, int(deletionCounter), time.Since(start))

	return nil
}

// deletes all bundles of which their tail tx doesn't exist in the database anymore.
func cleanupBundles() error {

	start := time.Now()

	var bundleCounter int64
	var deletionCounter int64
	tangle.ForEachBundleHash(func(tailTxHash hornet.Hash) bool {
		bundleCounter++

		if daemon.IsStopped() {
			return false
		}

		if (bundleCounter % 50000) == 0 {
			if daemon.IsStopped() {
				return false
			}
			log.Infof("analyzed %d bundles", bundleCounter)
		}

		storedTx := tangle.GetStoredTransactionOrNil(tailTxHash)

		// delete bundle if transaction doesn't exist
		if storedTx == nil {
			tangle.DeleteBundle(tailTxHash)
			deletionCounter++
			return true
		}

		return true
	})

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	log.Infof("%d bundles deleted, took %v", deletionCounter, time.Since(start))

	return nil
}
