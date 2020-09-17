package dag

import (
	"bytes"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// UpdateOutdatedRootSnapshotIndexes updates the transaction root snapshot indexes of the given transactions.
// the "outdatedTransactions" should be ordered from oldest to latest to avoid recursion.
func UpdateOutdatedRootSnapshotIndexes(outdatedMessageIDs hornet.Hashes, lsmi milestone.Index) {
	for _, outdatedTxHash := range outdatedMessageIDs {
		cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(outdatedTxHash)
		if cachedMsgMeta == nil {
			panic(tangle.ErrMessageNotFound)
		}
		GetTransactionRootSnapshotIndexes(cachedMsgMeta, lsmi)
	}
}

// GetTransactionRootSnapshotIndexes searches the transaction root snapshot indexes for a given transaction.
func GetTransactionRootSnapshotIndexes(cachedMessageMetadata *tangle.CachedMetadata, lsmi milestone.Index) (youngestTxRootSnapshotIndex milestone.Index, oldestTxRootSnapshotIndex milestone.Index) {
	defer cachedMessageMetadata.Release(true) // meta -1

	// if the tx already contains recent (calculation index matches LSMI)
	// information about ymrsi and omrsi, return that info
	ymrsi, omrsi, rtsci := cachedMessageMetadata.GetMetadata().GetRootSnapshotIndexes()
	if rtsci == lsmi {
		return ymrsi, omrsi
	}

	youngestTxRootSnapshotIndex = 0
	oldestTxRootSnapshotIndex = 0

	updateIndexes := func(ymrsi milestone.Index, omrsi milestone.Index) {
		if (youngestTxRootSnapshotIndex == 0) || (youngestTxRootSnapshotIndex < ymrsi) {
			youngestTxRootSnapshotIndex = ymrsi
		}
		if (oldestTxRootSnapshotIndex == 0) || (oldestTxRootSnapshotIndex > omrsi) {
			oldestTxRootSnapshotIndex = omrsi
		}
	}

	// collect all parents in the cone that are not confirmed,
	// are no solid entry points and have no recent calculation index
	var outdatedMessageIDs hornet.Hashes

	startMessageID := cachedMessageMetadata.GetMetadata().GetMessageID()

	indexesValid := true

	// traverse the parents of this message to calculate the root snapshot indexes for this message.
	// this walk will also collect all outdated messages in the same cone, to update them afterwards.
	if err := TraverseParents(cachedMessageMetadata.GetMetadata().GetMessageID(),
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMetadata *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedMetadata.Release(true) // meta -1

			// first check if the tx was confirmed => update ymrsi and omrsi with the confirmation index
			if confirmed, at := cachedMetadata.GetMetadata().GetConfirmed(); confirmed {
				updateIndexes(at, at)
				return false, nil
			}

			if bytes.Equal(startMessageID, cachedMetadata.GetMetadata().GetMessageID()) {
				return true, nil
			}

			// if the tx was not confirmed yet, but already contains recent (calculation index matches LSMI) information
			// about ymrsi and omrsi, propagate that info
			ymrsi, omrsi, rtsci := cachedMetadata.GetMetadata().GetRootSnapshotIndexes()
			if rtsci == lsmi {
				updateIndexes(ymrsi, omrsi)
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedMetadata *tangle.CachedMetadata) error { // meta +1
			defer cachedMetadata.Release(true) // meta -1

			if bytes.Equal(startMessageID, cachedMetadata.GetMetadata().GetMessageID()) {
				// skip the start transaction, so it doesn't get added to the outdatedMessageIDs
				return nil
			}

			outdatedMessageIDs = append(outdatedMessageIDs, cachedMetadata.GetMetadata().GetMessageID())
			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		func(txHash hornet.Hash) {
			// if the parent is a solid entry point, use the index of the solid entry point as ORTSI
			entryPointIndex, _ := tangle.SolidEntryPointsIndex(txHash)
			updateIndexes(entryPointIndex, entryPointIndex)
		}, false, nil); err != nil {
		if err == tangle.ErrMessageNotFound {
			indexesValid = false
		} else {
			panic(err)
		}
	}

	// update the outdated root snapshot indexes of all transactions in the cone in order from oldest txs to latest.
	// this is an efficient way to update the whole cone, because updating from oldest to latest will not be recursive.
	UpdateOutdatedRootSnapshotIndexes(outdatedMessageIDs, lsmi)

	// only set the calculated root snapshot indexes if all transactions in the past cone were found
	if !indexesValid {
		return 0, 0
	}

	// set the new transaction root snapshot indexes in the metadata of the transaction
	cachedMessageMetadata.GetMetadata().SetRootSnapshotIndexes(youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex, lsmi)

	return youngestTxRootSnapshotIndex, oldestTxRootSnapshotIndex
}

// UpdateMessageRootSnapshotIndexes updates the transaction root snapshot
// indexes of the future cone of all given transactions.
// all the transactions of the newly confirmed cone already have updated transaction root snapshot indexes.
// we have to walk the future cone, and update the past cone of all transactions that reference an old cone.
// as a special property, invocations of the yielded function share the same 'already traversed' set to circumvent
// walking the future cone of the same transactions multiple times.
func UpdateMessageRootSnapshotIndexes(txHashes hornet.Hashes, lsmi milestone.Index) {
	traversed := map[string]struct{}{}

	// we update all transactions in order from oldest to latest
	for _, txHash := range txHashes {

		if err := TraverseChildren(txHash,
			// traversal stops if no more transactions pass the given condition
			func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				_, previouslyTraversed := traversed[string(cachedMsgMeta.GetMetadata().GetMessageID())]
				return !previouslyTraversed, nil
			},
			// consumer
			func(cachedMsgMeta *tangle.CachedMetadata) error { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1
				traversed[string(cachedMsgMeta.GetMetadata().GetMessageID())] = struct{}{}

				// updates the transaction root snapshot indexes of the outdated past cone for this transaction
				GetTransactionRootSnapshotIndexes(cachedMsgMeta.Retain(), lsmi) // meta pass +1

				return nil
			}, false, nil); err != nil {
			panic(err)
		}
	}
}
