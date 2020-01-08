package snapshot

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	PruneUnconfirmedTransactionsDepth = 50
)

// pruneUnconfirmedTransactions prunes all unconfirmed tx from the database for the given milestone - PruneUnconfirmedTransactionsDepth
func pruneUnconfirmedTransactions(solidMilestoneIndex milestone_index.MilestoneIndex) {
	targetIndex := solidMilestoneIndex - PruneUnconfirmedTransactionsDepth

	log.Debugf("Pruning unconfirmed transactions older than milestone %d...", targetIndex)

	txHashes, err := tangle.ReadUnconfirmedTxHashOperations(targetIndex)
	if err != nil {
		log.Panicf("pruneUnconfirmedTransactions: %v", err.Error())
	}

	pruneTransactions(txHashes)

	if err := tangle.DeleteUnconfirmedTxHashOperations(txHashes); err != nil {
		log.Error(err)
	}

	log.Debugf("Pruned %d unconfirmed transactions", len(txHashes))
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
func pruneTransactions(txHashes []trinary.Hash) {

	txsToRemove := make(map[trinary.Hash]struct{})
	bundlesTxsToRemove := make(map[trinary.Hash]trinary.Hash)
	var approvers []*tangle.Approvers
	var addresses []*tangle.TxHashForAddress

	for _, txHash := range txHashes {
		tx, _ := tangle.GetTransaction(txHash)
		if tx == nil {
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		bundleBucket, err := tangle.GetBundleBucket(tx.Tx.Bundle)
		if err != nil {
			log.Panicf("pruneTransactions: Bundle bucket not found: %v", tx.Tx.Bundle)
		}

		for txToRemove := range bundleBucket.RemoveTransactionFromBundle(txHash) {
			txsToRemove[txToRemove] = struct{}{}
			bundlesTxsToRemove[tx.Tx.Bundle] = txToRemove
		}
	}

	for txHash := range txsToRemove {
		tx, _ := tangle.GetTransaction(txHash)
		if tx == nil {
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		approver, _ := tangle.GetApprovers(txHash)
		if approver == nil {
			continue
		}
		approvers = append(approvers, approver)

		addresses = append(addresses, &tangle.TxHashForAddress{TxHash: txHash, Address: tx.Tx.Address})

		tangle.DiscardApproversFromCache(txHash)
		tangle.DiscardTransactionFromCache(txHash)
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
	if err := tangle.DeleteTransactionsInDatabase(txsToRemove); err != nil {
		log.Error(err)
	}

	// address
	if err := tangle.DeleteTransactionHashesForAddressesInDatabase(addresses); err != nil {
		log.Error(err)
	}
}

// ToDo: Global pruning Lock needed?
func pruneDatabase(solidMilestoneIndex milestone_index.MilestoneIndex) {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	targetIndex := solidMilestoneIndex - pruningDelay
	if targetIndex > (snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - 1) {
		targetIndex = snapshotInfo.SnapshotIndex - SolidEntryPointCheckThresholdPast - 1
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// No pruning needed
		return
	}

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex <= targetIndex; milestoneIndex++ {
		log.Infof("Pruning milestone (%d)...", milestoneIndex)

		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			log.Panicf("Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		approvees, err := getMilestoneApprovees(milestoneIndex, ms.GetTail(), nil)
		if err != nil {
			log.Errorf("Pruning milestone (%d) failed! %v", milestoneIndex, err)
			continue
		}
		pruneTransactions(approvees)
		pruneMilestone(milestoneIndex)
		log.Infof("Pruning milestone (%d) done!", milestoneIndex)
	}

	snapshotInfo.PruningIndex = targetIndex
	tangle.SetSnapshotInfo(snapshotInfo)
}
