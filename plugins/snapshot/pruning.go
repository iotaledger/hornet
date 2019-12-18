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
	tangle.DeleteUnconfirmedTxHashOperations(txHashes)

	log.Debugf("Pruned %d unconfirmed transactions", len(txHashes))
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone_index.MilestoneIndex) {

	// state diffs
	tangle.DeleteLedgerDiffForMilestone(milestoneIndex)

	tangle.DiscardMilestoneFromCache(milestoneIndex)

	// milestone
	tangle.DeleteMilestoneInDatabase(milestoneIndex)
}

// pruneMilestone prunes the approvers, bundles, address and transaction metadata from the database for the given txHashes
func pruneTransactions(txHashes []trinary.Hash) {

	// Prune the approvers of all tx
	var approvers []*tangle.Approvers
	for _, txHash := range txHashes {
		approver, _ := tangle.GetApprovers(txHash)
		if approver == nil {
			continue
		}
		tangle.DiscardApproversFromCache(txHash)
		approvers = append(approvers, approver)
	}
	tangle.DeleteApproversInDatabase(approvers)

	var addresses []*tangle.TxHashForAddress
	bundles := make(map[trinary.Hash]trinary.Hash)
	for _, txHash := range txHashes {
		tx, _ := tangle.GetTransaction(txHash)
		if tx == nil {
			log.Panicf("pruneTransactions: Transaction not found: %v", txHash)
		}

		if tx.IsTail() {
			bundleBucket, err := tangle.GetBundleBucket(tx.Tx.Bundle)
			if err != nil {
				log.Panicf("pruneTransactions: Bundle bucket not found: %v", tx.Tx.Bundle)
			}

			bundle := bundleBucket.GetBundleOfTailTransaction(txHash)
			if bundle == nil {
				log.Panicf("pruneTransactions: Bundle not found: TailHash: %v", txHash)
			}

			for _, bundleTxHash := range bundle.GetTransactionHashes() {
				bundles[tx.Tx.Bundle] = bundleTxHash
			}

			bundleBucket.RemoveBundleByTailTxHash(txHash)
		}

		addresses = append(addresses, &tangle.TxHashForAddress{TxHash: txHash, Address: tx.Tx.Address})
		tangle.DiscardTransactionFromCache(txHash)
	}

	// bundles
	tangle.DeleteBundlesInDatabase(bundles)

	// tx
	tangle.DeleteTransactionsInDatabase(txHashes)

	// address
	tangle.DeleteTransactionHashesForAddressesInDatabase(addresses)
}

// ToDo: Global pruning Lock needed?
func pruneDatabase(solidMilestoneIndex milestone_index.MilestoneIndex) {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	targetIndex := solidMilestoneIndex - pruningDelay
	if targetIndex > (solidMilestoneIndex - snapshotDepth - SolidEntryPointCheckThreshold) {
		targetIndex = solidMilestoneIndex - snapshotDepth - SolidEntryPointCheckThreshold
	}

	if snapshotInfo.PruningIndex >= targetIndex {
		// No pruning needed
		return
	}

	// Iterate through all milestones that have to be pruned
	for milestoneIndex := snapshotInfo.PruningIndex + 1; milestoneIndex < targetIndex; milestoneIndex++ {

		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			log.Panicf("Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		approvees := getApprovees(milestoneIndex, ms.GetTail())
		pruneTransactions(approvees)
		pruneMilestone(milestoneIndex)
	}

	snapshotInfo.PruningIndex = targetIndex
	tangle.SetSnapshotInfo(snapshotInfo)
}
