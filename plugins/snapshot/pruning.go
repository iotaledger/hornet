package snapshot

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/dag"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	// AdditionalPruningThreshold is needed, because the transactions in the getMilestoneApprovees call in getSolidEntryPoints
	// can reference older transactions as well
	AdditionalPruningThreshold = 50
)

// pruneUnconfirmedTransactions prunes all unconfirmed tx from the database for the given milestone
func pruneUnconfirmedTransactions(targetIndex milestone_index.MilestoneIndex) int {

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
func pruneMilestone(milestoneIndex milestone_index.MilestoneIndex) {

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
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
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
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}
		tangle.DeleteTag(cachedTx.GetTransaction().Tx.Tag, txHash)
		tangle.DeleteAddress(cachedTx.GetTransaction().Tx.Address, txHash)
		tangle.DeleteApprovers(txHash)
		tangle.DeleteTransaction(txHash)
		cachedTx.Release(true) // tx -1
	}

	return len(txsToRemove)
}

// ToDo: Global pruning Lock needed?
func pruneDatabase(solidMilestoneIndex milestone_index.MilestoneIndex, abortSignal <-chan struct{}) {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	targetIndex := solidMilestoneIndex - pruningDelay
	targetIndexMax := (snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - AdditionalPruningThreshold - 1)
	if targetIndex > targetIndexMax {
		targetIndex = targetIndexMax
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// No pruning needed
		return
	}

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			// Stop pruning the next milestone
			return
		default:
		}

		log.Infof("Pruning milestone (%d)...", milestoneIndex)

		ts := time.Now()
		txCount := pruneUnconfirmedTransactions(milestoneIndex)

		cachedMs := tangle.GetMilestoneOrNil(milestoneIndex) // bundle +1
		if cachedMs == nil {
			log.Panicf("Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		cachedMsTailTx := cachedMs.GetBundle().GetTail() // tx +1
		cachedMs.Release(true)                           // bundle -1

		tsmw := time.Now()
		approvees, err := dag.GetMilestoneApprovees(milestoneIndex, cachedMsTailTx.Retain(), true, nil)
		log.Debugf("Milestone walked (%d): approvees: %v, collect: %v", milestoneIndex, len(approvees), time.Since(tsmw))

		// Do not force release, since it is loaded again for pruning
		cachedMsTailTx.Release() // tx -1

		if err != nil {
			log.Errorf("Pruning milestone (%d) failed! %v", milestoneIndex, err)
			continue
		}

		txCount += pruneTransactions(approvees)

		pruneMilestone(milestoneIndex)

		log.Infof("Pruning milestone (%d) took %v. Pruned %d transactions. ", milestoneIndex, time.Since(ts), txCount)
	}

	snapshotInfo.PruningIndex = targetIndex
	tangle.SetSnapshotInfo(snapshotInfo)
}
