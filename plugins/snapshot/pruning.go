package snapshot

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

const (
	// AdditionalPruningThreshold is needed, because the transactions in the getMilestoneApprovees call in getSolidEntryPoints
	// can reference older transactions as well
	AdditionalPruningThreshold = 50
)

// pruneUnconfirmedTransactions prunes all unconfirmed tx from the database for the given milestone
func pruneUnconfirmedTransactions(targetIndex milestone.Index) int {

	txsToRemoveMap := make(map[trinary.Hash]struct{})
	var txsToRemoveSlice []trinary.Hash

	// Check if tx is still unconfirmed
	for _, txHash := range tangle.GetFirstSeenTxHashes(targetIndex, true) {
		cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
		if cachedTx == nil {
			// Tx was already pruned
			continue
		}

		if confirmed, _ := cachedTx.GetMetadata().GetConfirmed(); confirmed {
			// Tx was confirmed => skip
			cachedTx.Release(true) // tx -1
			continue
		}

		if _, exists := txsToRemoveMap[txHash]; exists {
			cachedTx.Release(true) // tx -1
			continue
		}

		txsToRemoveMap[txHash] = struct{}{}
		txsToRemoveSlice = append(txsToRemoveSlice, txHash)

		// Do not force release, since it is loaded again for pruning
		cachedTx.Release() // tx -1
	}

	txCount := pruneTransactions(txsToRemoveSlice)
	tangle.DeleteFirstSeenTxs(targetIndex)

	return txCount
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) {

	// state diffs
	if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
		log.Error(err)
	}

	tangle.DeleteMilestone(milestoneIndex)
}

// pruneMilestone prunes the approvers, bundles, addresses and transaction metadata from the database
// if the given txHashes are removed from their corresponding bundles
func pruneTransactions(txHashes []trinary.Hash) int {

	txsToRemove := make(map[trinary.Hash]struct{})

	for _, txHash := range txHashes {
		cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
		if cachedTx == nil {
			log.Warnf("pruneTransactions: Transaction not found: %v", txHash)
			continue
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(cachedTx.GetTransaction().Tx) {
			txsToRemove[txToRemove] = struct{}{}
		}

		// Do not force release, since it is loaded again for pruning
		cachedTx.Release() // tx -1
	}

	for txHash := range txsToRemove {
		cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
		if cachedTx == nil {
			log.Warnf("pruneTransactions: Transaction not found: %v", txHash)
			continue
		}

		// Delete the reference in the approvees
		tangle.DeleteApprover(cachedTx.GetTransaction().GetTrunk(), txHash)
		tangle.DeleteApprover(cachedTx.GetTransaction().GetBranch(), txHash)

		tangle.DeleteTag(cachedTx.GetTransaction().Tx.Tag, txHash)
		tangle.DeleteAddress(cachedTx.GetTransaction().Tx.Address, txHash)
		tangle.DeleteApprovers(txHash)
		tangle.DeleteTransaction(txHash)
		cachedTx.Release(true) // tx -1
	}

	return len(txsToRemove)
}

func pruneDatabase(targetIndex milestone.Index, abortSignal <-chan struct{}) error {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	if snapshotInfo.SnapshotIndex < SolidEntryPointCheckThresholdPast+AdditionalPruningThreshold+1 {
		// Not enough history
		return ErrNotEnoughHistory
	}

	targetIndexMax := snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - AdditionalPruningThreshold - 1
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// No pruning needed
		return ErrNoPruningNeeded
	}

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			// Stop pruning the next milestone
			return ErrPruningAborted
		default:
		}

		log.Infof("Pruning milestone (%d)...", milestoneIndex)

		ts := time.Now()
		txCount := pruneUnconfirmedTransactions(milestoneIndex)

		cachedMs := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMs == nil {
			// Milestone not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone not found!", milestoneIndex)
			continue
		}

		// Get all approvees of that milestone
		cachedMsTailTx := tangle.GetCachedTransactionOrNil(cachedMs.GetMilestone().Hash) // tx +1
		cachedMs.Release(true)                                                           // milestone -1

		if cachedMsTailTx == nil {
			// Milestone tail not found, pruning impossible
			log.Warnf("Pruning milestone (%d) failed! Milestone tail tx not found!", milestoneIndex)
			continue
		}
		approvees, err := getMilestoneApprovees(milestoneIndex, cachedMsTailTx.Retain(), true, nil)

		// Do not force release, since it is loaded again for pruning
		cachedMsTailTx.Release() // tx -1

		if err != nil {
			log.Warnf("Pruning milestone (%d) failed! Error: %v", milestoneIndex, err)
			continue
		}

		txCount += pruneTransactions(approvees)

		pruneMilestone(milestoneIndex)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d transactions. ", milestoneIndex, time.Since(ts), txCount)
	}

	snapshotInfo.PruningIndex = targetIndex
	tangle.SetSnapshotInfo(snapshotInfo)

	tanglePlugin.Events.PruningMilestoneIndexChanged.Trigger(targetIndex)

	return nil
}
