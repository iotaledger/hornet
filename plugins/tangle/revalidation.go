package tangle

import (
	"errors"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	RevalidationMilestoneThreshold = milestone_index.MilestoneIndex(50)
)

var (
	ErrLatestMilestoneOlderThanSnapshotIndex = errors.New("Latest milestone in the database is older than the snapshot index")
	ErrSnapshotIndexWrong                    = errors.New("Snapshot index does not fit the snapshot ledger index")
)

// revalidateDatabase tries to revalidate a corrupted database (after an unclean node shutdown/crash)
//
// HORNET uses caches for almost all tangle related data.
// If the node crashes, it is not guaranteed that all data in the cache was already persisted to the disk.
// Thats why we flag the database as corrupted.
//
// This function tries to restore a clean database state by searching all existing milestones
// since last local snapshot, deleting all ledger states and changes, loading valid snapshot ledger state,
// and reapplying all known milestones.
//
// It returns a milestone index, which is later used by the solidifier to revalidate all transaction
// related data (metadata, tags, addresses, bundles etc.).
// The solidifier will ignore all metadata (solid/confirmed flags, confirmation index) of cones that are older
// than this milestone index. Additional information of the transactions (Addresses, Tags, etc) will
// be reapplied by the solidifer.
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
//			- FirstSeenTx 						=> will be removed at pruning anyway
//			- Milestone							=> will be added again at bundle creation if missing
//			- SpentAddresses					=> will be added again if missing by confirmation
//
// Database:
// 		- LedgerState
//			- Balances of latest solid milestone		=> will be removed and replaced with snapshot milestone
//			- Balances of snapshot milestone			=> should be consistent (total iotas are checked)
//			- Balance diffs of every solid milestone	=> will be removed and added again by confirmation
//
func revalidateDatabase() (milestone_index.MilestoneIndex, error) {

	snapshotInfo := tangle.GetSnapshotInfo()
	latestMilestoneIndex := tangle.SearchLatestMilestoneIndex()

	// Resume old revalidation attempts
	if snapshotInfo.RevalidationIndex != 0 && latestMilestoneIndex < (snapshotInfo.RevalidationIndex-RevalidationMilestoneThreshold) {
		latestMilestoneIndex = snapshotInfo.RevalidationIndex - RevalidationMilestoneThreshold
	}

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return 0, ErrLatestMilestoneOlderThanSnapshotIndex
	}

	// It has to be stored in the snapshot info, otherwise a failed revalidation attempt would lead to missing info about latestMilestoneIndex
	snapshotInfo.RevalidationIndex = latestMilestoneIndex + RevalidationMilestoneThreshold
	tangle.SetSnapshotInfo(snapshotInfo)

	// Walk all milestones since SnapshotIndex and delete all corrupted balances diffs and milestone bundles
	// Add a treshold in case the milestones don't exist, but the ledger data was stored already
	for milestoneIndex := snapshotInfo.SnapshotIndex + 1; milestoneIndex <= snapshotInfo.RevalidationIndex; milestoneIndex++ {
		if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
			return 0, err
		}
	}

	// Walk all milestones since SnapshotIndex and try to reconstruct all known milestones
	for milestoneIndex := snapshotInfo.SnapshotIndex + 1; milestoneIndex <= latestMilestoneIndex; milestoneIndex++ {

		cachedMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			// Maybe node was not solid and never received the milestone
			continue
		}
		milestoneTailTxHash := cachedMilestone.GetMilestone().Hash
		cachedMilestone.Release(true)

		// Delete the information about this milestone (it will be reconstructed if all data is available)
		tangle.DeleteMilestone(milestoneIndex)

		// Delete the milestone bundle (it will be reconstructed if all data is available)
		tangle.DeleteBundle(milestoneTailTxHash)

		cachedMsTailTx := tangle.GetCachedTransactionOrNil(milestoneTailTxHash) // tx +1
		if cachedMsTailTx == nil {
			// Transaction not available => skip milestone bundle reconstruction
			continue
		}

		// Try to reconstruct the milestone bundle
		// If all tx are available, it will succeed, otherwise the bundle will be constructed if the missing tx are received
		tangle.TryConstructBundle(cachedMsTailTx.Retain(), false)
		cachedMsTailTx.Release(true) // tx -1
	}

	// Get the ledger state of the last snapshot
	snapshotBalances, snapshotIndex, err := tangle.GetAllSnapshotBalances(nil)
	if err != nil {
		return 0, err
	}

	if snapshotInfo.SnapshotIndex != snapshotIndex {
		return 0, ErrSnapshotIndexWrong
	}

	// Delete the corrupted ledger state
	if err = tangle.DeleteLedgerBalancesInDatabase(); err != nil {
		return 0, err
	}

	// Store the snapshot balances as the current valid ledger
	if err = tangle.StoreLedgerBalancesInDatabase(snapshotBalances, snapshotIndex); err != nil {
		return 0, err
	}

	// Add a treshold in case the milestones don't exist, but parts of confirmed cones were already stored
	return snapshotInfo.RevalidationIndex, nil
}
