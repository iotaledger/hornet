package tangle

import (
	"bytes"
	"errors"
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/iotaledger/hive.go/typeutils"
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
//			- TxRaw (synced)					=> will be added again by requesting the tx at solidification
//			- TxMetadata (synced)				=> will be removed and added again by solidifcation
//			- BundleTransaction (synced)		=> will be added again if missing by solidifcation
//			- Bundle (always)					=> will be removed and added again by solidifcation
//			- Approver (synced)					=> will be added again if missing by solidifcation
//
// 		Stored without caching:
//			- Tag								=> will be added again if missing by solidifcation
//			- Address							=> will be added again if missing by solidifcation
//			- UnconfirmedTx 						=> will be removed at pruning anyway
//			- Milestone							=> will be added again at bundle creation if missing
//			- SpentAddresses					=> will be added again if missing by confirmation
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

	latestMilestoneIndex := tangle.SearchLatestMilestoneIndexInBadger()

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	log.Infof("reverting database state back to local snapshot %d...", snapshotInfo.SnapshotIndex)

	// delete milestone data newer than the local snapshot
	cleanMilestones(snapshotInfo)

	// clean up transactions which are above the local snapshot
	cleanupTransactions(snapshotInfo)

	// Get the ledger state of the last snapshot
	snapshotBalances, snapshotIndex, err := tangle.GetAllSnapshotBalances(nil)
	if err != nil {
		return err
	}

	if snapshotInfo.SnapshotIndex != snapshotIndex {
		return ErrSnapshotIndexWrong
	}

	// Delete the corrupted ledger state
	if err = tangle.DeleteLedgerBalancesInDatabase(); err != nil {
		return err
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
func cleanMilestones(info *tangle.SnapshotInfo) {
	milestonesToDelete := map[milestone.Index]struct{}{}

	tangle.ForEachMilestoneIndex(func(msIndex milestone.Index) {
		if msIndex > info.SnapshotIndex {
			milestonesToDelete[msIndex] = struct{}{}
		}
	}, true)

	for msIndex := range milestonesToDelete {
		tangle.DeleteUnconfirmedTxsFromBadger(msIndex)
		if err := tangle.DeleteLedgerDiffForMilestone(msIndex); err != nil {
			panic(err)
		}
		tangle.DeleteMilestoneFromBadger(msIndex)
	}
}

// deletes all transactions (and their bundle, first seen tx, etc.) which are not confirmed,
// not solid, their confirmation milestone is newer/ of which their solidification time is younger
// than the last local snapshot's milestone.
func cleanupTransactions(info *tangle.SnapshotInfo) {
	txsToDelete := map[string]struct{}{}

	start := time.Now()
	var txCounter int64
	tangle.ForEachTransactionHashBytes(func(txHashBytes []byte) {

		txCounter++

		if (txCounter % 50000) == 0 {
			log.Infof("analyzed %d transactions", txCounter)
		}

		storedTxMeta := tangle.GetStoredMetadataOrNil(txHashBytes)

		// delete transaction if no metadata
		if storedTxMeta == nil {
			txsToDelete[typeutils.BytesToString(txHashBytes)] = struct{}{}
			return
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			txsToDelete[typeutils.BytesToString(txHashBytes)] = struct{}{}
			return
		}

		if confirmed, by := storedTxMeta.GetConfirmed(); !confirmed || by > info.SnapshotIndex {
			txsToDelete[typeutils.BytesToString(txHashBytes)] = struct{}{}
			return
		}
	})

	var deletionCounter float64
	total := float64(len(txsToDelete))
	lastPercentage := 0
	nullHashBytes := make([]byte, 49)
	for txHashToDelete := range txsToDelete {
		deletionCounter++

		txHashBytesToDelete := typeutils.StringToBytes(txHashToDelete)
		if bytes.Equal(txHashBytesToDelete, nullHashBytes) {
			// do not delete genesis transaction
			continue
		}

		percentage := int((deletionCounter / total) * 100)
		if lastPercentage+5 <= percentage {
			lastPercentage = percentage
			log.Infof("reverting (this might take a while)...%d/%d (%d%%)", int(deletionCounter), len(txsToDelete), percentage)
		}

		storedTx := tangle.GetStoredTransactionOrNil(txHashBytesToDelete)
		if storedTx == nil {
			continue
		}
		// No need to safely remove the transactions from the bundle,
		// since reattachment txs confirmed by another milestone wouldn't be
		// pruned anyway if they are confirmed before snapshot index.
		tangle.DeleteBundleTransactionFromBadger(storedTx.Tx.Bundle, txHashBytesToDelete, true)
		tangle.DeleteBundleTransactionFromBadger(storedTx.Tx.Bundle, txHashBytesToDelete, false)
		tangle.DeleteBundleFromBadger(txHashBytesToDelete)

		// Delete the reference in the approvees
		tangle.DeleteApproverFromBadger(storedTx.GetTrunk(), txHashBytesToDelete)
		tangle.DeleteApproverFromBadger(storedTx.GetBranch(), txHashBytesToDelete)

		tangle.DeleteTagFromBadger(storedTx.Tx.Tag, txHashBytesToDelete)
		tangle.DeleteAddressFromBadger(storedTx.Tx.Address, txHashBytesToDelete)
		tangle.DeleteApproversFromBadger(txHashBytesToDelete)
		tangle.DeleteTransactionFromBadger(txHashBytesToDelete)
	}

	log.Infof("reverted state back to local snapshot %d, %d transactions deleted, took %v", info.SnapshotIndex, int(deletionCounter), time.Since(start))
}
