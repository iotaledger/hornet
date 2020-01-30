package snapshot

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

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

	txHashes, err := tangle.ReadFirstSeenTxHashOperations(targetIndex)
	if err != nil {
		log.Panicf("pruneUnconfirmedTransactions: %v", err.Error())
	}

	txsToRemoveMap := make(map[trinary.Hash]struct{})
	var txsToRemoveSlice []trinary.Hash

	// Check if tx is still unconfirmed
	for _, txHash := range txHashes {
		tx := tangle.GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			// Tx was already pruned
			tx.Release() //-1
			continue
		}

		if confirmed, _ := tx.GetTransaction().GetConfirmed(); confirmed {
			// Tx was confirmed => skip
			tx.Release() //-1
			continue
		}

		if _, exists := txsToRemoveMap[txHash]; exists {
			tx.Release() //-1
			continue
		}

		txsToRemoveMap[txHash] = struct{}{}
		txsToRemoveSlice = append(txsToRemoveSlice, txHash)
		tx.Release() //-1
	}

	txCount := pruneTransactions(txsToRemoveSlice)

	if err := tangle.DeleteFirstSeenTxHashOperations(targetIndex); err != nil {
		log.Error(err)
	}

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
// if the given txHashes are removed from their corresponding bundle buckets
func pruneTransactions(txHashes []trinary.Hash) int {

	txsToRemove := make(map[trinary.Hash]struct{})
	var addresses []*tangle.TxHashForAddress

	for _, txHash := range txHashes {
		tx := tangle.GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			tx.Release() //-1
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(tx.GetTransaction().Tx.Bundle, txHash) {
			txsToRemove[txToRemove] = struct{}{}
		}
		tx.Release() //-1
	}

	for txHash := range txsToRemove {
		tx := tangle.GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			tx.Release() //-1
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		addresses = append(addresses, &tangle.TxHashForAddress{TxHash: txHash, Address: tx.GetTransaction().Tx.Address})
		tx.Release() //-1

		tangle.DeleteApprovers(txHash)
		tangle.DeleteTransaction(txHash)
	}

	// address
	if err := tangle.DeleteTransactionHashesForAddressesInDatabase(addresses); err != nil {
		log.Error(err)
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

		ms := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			log.Panicf("Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		msTail := ms.GetTail() //+1
		approvees, err := getMilestoneApprovees(milestoneIndex, msTail, false, nil)
		msTail.Release() //-1
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
