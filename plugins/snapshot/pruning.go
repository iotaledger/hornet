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

	tangle.DiscardMilestoneFromCache(milestoneIndex)

	// milestone
	if err := tangle.DeleteMilestoneInDatabase(milestoneIndex); err != nil {
		log.Error(err)
	}
}

// pruneMilestone prunes the approvers, bundles, addresses and transaction metadata from the database
// if the given txHashes are removed from their corresponding bundle buckets
func pruneTransactions(txHashes []trinary.Hash) int {

	txsToRemove := make(map[trinary.Hash]struct{})
	bundlesTxsToRemove := make(map[trinary.Hash]trinary.Hash)
	var approvers []*tangle.Approvers
	var addresses []*tangle.TxHashForAddress

	for _, txHash := range txHashes {
		tx := tangle.GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			tx.Release() //-1
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		bundleBucket, err := tangle.GetBundleBucket(tx.GetTransaction().Tx.Bundle)
		if err != nil {
			log.Panicf("pruneTransactions: Bundle bucket not found: %v", tx.GetTransaction().Tx.Bundle)
			tx.Release() //-1
		}

		for txToRemove := range bundleBucket.RemoveTransactionFromBundle(txHash) {
			txsToRemove[txToRemove] = struct{}{}
			bundlesTxsToRemove[tx.GetTransaction().Tx.Bundle] = txToRemove
		}
		tx.Release()
	}

	for txHash := range txsToRemove {
		tx := tangle.GetCachedTransaction(txHash) //+1
		if !tx.Exists() {
			tx.Release() //-1
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		approver, _ := tangle.GetApprovers(txHash)
		if approver == nil {
			tx.Release() //-1
			continue
		}
		approvers = append(approvers, approver)

		addresses = append(addresses, &tangle.TxHashForAddress{TxHash: txHash, Address: tx.GetTransaction().Tx.Address})

		tangle.DiscardApproversFromCache(txHash)
		tangle.DiscardTransaction(txHash)
		tx.Release() //-1
	}

	// approvers
	if err := tangle.DeleteApproversInDatabase(approvers); err != nil {
		log.Error(err)
	}

	// bundles
	if err := tangle.DeleteBundlesInDatabase(bundlesTxsToRemove); err != nil {
		log.Error(err)
	}

	// tx
	for txToRemove := range txsToRemove {
		tangle.DiscardTransaction(txToRemove)
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

		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			log.Panicf("Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		approvees, err := getMilestoneApprovees(milestoneIndex, ms.GetTail(), false, nil)
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
