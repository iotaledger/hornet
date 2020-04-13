package tangle

import (
	"errors"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
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

	latestMilestoneIndex := tangle.SearchLatestMilestoneIndex()

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
	tangle.SetSolidMilestoneIndex(snapshotInfo.SnapshotIndex)

	return nil
}

// holds data about a transaction which should be deleted
type deletionMeta struct {
	bundle  trinary.Hash
	address trinary.Hash
	tag     trinary.Trytes
}

func deletionMetaFor(tx *hornet.Transaction) deletionMeta {
	return deletionMeta{
		bundle:  tx.Tx.Bundle,
		address: tx.Tx.Address,
		tag:     tx.Tx.Tag,
	}
}

// deletes milestones above the given snapshot's milestone index.
func cleanMilestones(info *tangle.SnapshotInfo) {
	milestonesToDelete := map[milestone.Index]struct{}{}
	tangle.ForEachMilestone(func(cachedMs objectstorage.CachedObject) {
		defer cachedMs.Release(true)
		msIndex := cachedMs.Get().(*tangle.Milestone).Index
		if msIndex > info.SnapshotIndex {
			milestonesToDelete[msIndex] = struct{}{}
			tangle.DeleteUnconfirmedTxs(msIndex)
			if err := tangle.DeleteLedgerDiffForMilestone(msIndex); err != nil {
				panic(err)
			}
		}
	})

	for index := range milestonesToDelete {
		tangle.DeleteMilestone(index)
	}

	tangle.FlushUnconfirmedTxsStorage()
	tangle.FlushMilestoneStorage()
}

// deletes all transactions (and their bundle, first seen tx, etc.) which are not confirmed,
// not solid, their confirmation milestone is newer/ of which their solidification time is younger
// than the last local snapshot's milestone.
func cleanupTransactions(info *tangle.SnapshotInfo) {
	txsToDelete := map[string]deletionMeta{}
	start := time.Now()
	var txCounter int64
	tangle.ForEachTransaction(func(cachedTx objectstorage.CachedObject, cachedTxMeta objectstorage.CachedObject) {
		defer cachedTx.Release(true) // tx -1
		tx := cachedTx.Get().(*hornet.Transaction)

		txCounter++
		fmt.Printf("analyzed %d transactions\t\t\r", txCounter)

		// delete transaction if no metadata
		if cachedTxMeta == nil {
			txsToDelete[tx.GetHash()] = deletionMetaFor(tx)
			return
		} else {
			defer cachedTxMeta.Release(true) // tx meta -1
		}

		txMeta := cachedTxMeta.Get().(*hornet.TransactionMetadata)

		// not solid
		if !txMeta.IsSolid() {
			txsToDelete[tx.GetHash()] = deletionMetaFor(tx)
			return
		}

		if confirmed, by := txMeta.GetConfirmed(); !confirmed || by > info.SnapshotIndex {
			txsToDelete[tx.GetHash()] = deletionMetaFor(tx)
			return
		}
	})

	var deletionCounter float64
	total := float64(len(txsToDelete))
	for txToDeleteHash, meta := range txsToDelete {
		deletionCounter++
		if txToDeleteHash == consts.NullHashTrytes {
			continue
		}
		fmt.Printf("reverting (this might take a while)... %d%%\t\t\r", int((deletionCounter/total)*100))
		tangle.DeleteBundleTransaction(meta.bundle, txToDeleteHash, true)
		tangle.DeleteBundleTransaction(meta.bundle, txToDeleteHash, false)
		tangle.DeleteBundle(txToDeleteHash)
		tangle.DeleteAddress(meta.address, txToDeleteHash)
		tangle.DeleteTag(meta.tag, txToDeleteHash)
		tangle.DeleteApprovers(txToDeleteHash)
		tangle.DeleteTransaction(txToDeleteHash)
	}

	// flush object storage
	tangle.FlushBundleStorage()
	tangle.FlushBundleTransactionsStorage()
	tangle.FlushTagsStorage()
	tangle.FlushAddressStorage()
	tangle.FlushApproversStorage()
	tangle.FlushTransactionStorage()

	log.Infof("reverted state back to local snapshot %d, took %v", info.SnapshotIndex, time.Since(start))
}
