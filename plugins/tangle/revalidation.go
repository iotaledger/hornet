package tangle

import (
	"errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

const (
	RevalidationMilestoneThreshold = milestone.Index(50)
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
func revalidateDatabase() (milestone.Index, error) {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		return 0, ErrSnapshotInfoMissing
	}

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

	// Walk all milestones since SnapshotIndex and delete all corrupted balances diffs and milestones.
	// Add a treshold in case the milestones don't exist, but the ledger data was stored already.
	// Existing milestone bundles or transactions don't have to be deleted. Their metadata will be resetted or ignored
	// during the solidification walk, and milestones will be reapplied to the database.
	for milestoneIndex := snapshotInfo.SnapshotIndex + 1; milestoneIndex <= snapshotInfo.RevalidationIndex; milestoneIndex++ {
		// Delete the information about this milestone (it will be reapplied during solidification walk)
		tangle.DeleteMilestone(milestoneIndex)

		if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
			return 0, err
		}
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

	// Set the valid solid milestone index
	tangle.SetSolidMilestoneIndex(snapshotInfo.SnapshotIndex)

	// Add a treshold in case the milestones don't exist, but parts of confirmed cones were already stored
	return snapshotInfo.RevalidationIndex, nil
}
