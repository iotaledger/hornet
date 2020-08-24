package dag

import (
	"bytes"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// UpdateOutdatedRootSnapshotIndexes updates the transaction root snapshot indexes of the given transactions.
// the "outdatedTransactions" should be ordered from oldest to latest to avoid recursion.
func UpdateOutdatedRootSnapshotIndexes(outdatedTransactions hornet.Hashes, lsmi milestone.Index) {
	for _, outdatedTxHash := range outdatedTransactions {
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(outdatedTxHash)
		if cachedTxMeta == nil {
			panic(tangle.ErrTransactionNotFound)
		}
		GetTransactionRootSnapshotIndexes(cachedTxMeta, lsmi)
	}
}

// GetTransactionRootSnapshotIndexes searches the transaction root snapshot indexes for a given transaction.
func GetTransactionRootSnapshotIndexes(cachedTxMeta *tangle.CachedMetadata, lsmi milestone.Index) (youngestTxRootSnapshotIndex milestone.Index, oldestTxRootSnapshotIndex milestone.Index) {
	defer cachedTxMeta.Release(true) // meta -1

	// if the tx already contains recent (calculation index matches LSMI)
	// information about yrtsi and ortsi, return that info
	yrtsi, ortsi, rtsci := cachedTxMeta.GetMetadata().GetRootSnapshotIndexes()
	if rtsci == lsmi {
		return yrtsi, ortsi
	}

	youngestTxRootSnapshotIndex = 0
	oldestTxRootSnapshotIndex = 0

	updateIndexes := func(yrtsi milestone.Index, ortsi milestone.Index) {
		if (youngestTxRootSnapshotIndex == 0) || (youngestTxRootSnapshotIndex < yrtsi) {
			youngestTxRootSnapshotIndex = yrtsi
		}
		if (oldestTxRootSnapshotIndex == 0) || (oldestTxRootSnapshotIndex > ortsi) {
			oldestTxRootSnapshotIndex = ortsi
		}
	}

	// collect all approvees in the cone that are not confirmed,
	// are no solid entry points and have no recent calculation index
	var outdatedTransactions hornet.Hashes

	startTxHash := cachedTxMeta.GetMetadata().GetTxHash()

	indexesValid := true

	// traverse the approvees of this transaction to calculate the root snapshot indexes for this transaction.
	// this walk will also collect all outdated transactions in the same cone, to update them afterwards.
	if err := TraverseApprovees(cachedTxMeta.GetMetadata().GetTxHash(),
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			// first check if the tx was confirmed => update yrtsi and ortsi with the confirmation index
			if confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed(); confirmed {
				updateIndexes(at, at)
				return false, nil
			}

			if bytes.Equal(startTxHash, cachedTxMeta.GetMetadata().GetTxHash()) {
				return true, nil
			}

			// if the tx was not confirmed yet, but already contains recent (calculation index matches LSMI) information
			// about yrtsi and ortsi, propagate that info
			yrtsi, ortsi, rtsci := cachedTxMeta.GetMetadata().GetRootSnapshotIndexes()
			if rtsci == lsmi {
				updateIndexes(yrtsi, ortsi)
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			if bytes.Equal(startTxHash, cachedTxMeta.GetMetadata().GetTxHash()) {
				// skip the start transaction, so it doesn't get added to the outdatedTransactions
				return nil
			}

			outdatedTransactions = append(outdatedTransactions, cachedTxMeta.GetMetadata().GetTxHash())
			return nil
		},
		// called on missing approvees
		func(approveeHash hornet.Hash) error {
			// since this is also called for the future cone, there may be missing approvees
			return tangle.ErrTransactionNotFound
		},
		// called on solid entry points
		func(txHash hornet.Hash) {
			// if the approvee is a solid entry point, use the index of the solid entry point as ORTSI
			entryPointIndex, _ := tangle.SolidEntryPointsIndex(txHash)
			updateIndexes(entryPointIndex, entryPointIndex)
		}, false, false, nil); err != nil {
		if err == tangle.ErrTransactionNotFound {
			indexesValid = false
		} else {
			panic(err)
		}
	}

	// update the outdated root snapshot indexes of all transactions in the cone in order from oldest txs to latest.
	// this is an efficient way to update the whole cone, because updating from oldest to latest will not be recursive.
	UpdateOutdatedRootSnapshotIndexes(outdatedTransactions, lsmi)

	// only set the calculated root snapshot indexes if all transactions in the past cone were found
	if !indexesValid {
		return 0, 0
	}

	// set the new transaction root snapshot indexes in the metadata of the transaction
	cachedTxMeta.GetMetadata().SetRootSnapshotIndexes(youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex, lsmi)

	return youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex
}

// UpdateTransactionRootSnapshotIndexes updates the transaction root snapshot
// indexes of the future cone of all given transactions.
// all the transactions of the newly confirmed cone already have updated transaction root snapshot indexes.
// we have to walk the future cone, and update the past cone of all transactions that reference an old cone.
// as a special property, invocations of the yielded function share the same 'already traversed' set to circumvent
// walking the future cone of the same transactions multiple times.
func UpdateTransactionRootSnapshotIndexes(txHashes hornet.Hashes, lsmi milestone.Index) {
	traversed := map[string]struct{}{}

	// we update all transactions in order from oldest to latest
	for _, txHash := range txHashes {

		if err := TraverseApprovers(txHash,
			// traversal stops if no more transactions pass the given condition
			func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedTxMeta.Release(true) // meta -1
				_, previouslyTraversed := traversed[string(cachedTxMeta.GetMetadata().GetTxHash())]
				return !previouslyTraversed, nil
			},
			// consumer
			func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
				defer cachedTxMeta.Release(true) // meta -1
				traversed[string(cachedTxMeta.GetMetadata().GetTxHash())] = struct{}{}

				// updates the transaction root snapshot indexes of the outdated past cone for this transaction
				GetTransactionRootSnapshotIndexes(cachedTxMeta.Retain(), lsmi) // meta pass +1

				return nil
			}, false, nil); err != nil {
			panic(err)
		}
	}
}
