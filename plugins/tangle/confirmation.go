package tangle

import (
	"time"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

// confirmMilestone traverses a milestone and collects all unconfirmed tx,
// then the ledger diffs are calculated, the ledger state is checked and all tx are marked as confirmed.
func confirmMilestone(milestoneIndex milestone_index.MilestoneIndex, milestoneTail *tangle.CachedTransaction) {

	milestoneTail.RegisterConsumer() //+1
	defer milestoneTail.Release()    //-1

	ts := time.Now()

	txsToConfirm := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	totalLedgerChanges := make(map[string]int64)
	txsToTraverse[milestoneTail.GetTransaction().GetHash()] = struct{}{}

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsToConfirm[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			tx := tangle.GetCachedTransaction(txHash) //+1
			if !tx.Exists() {
				log.Panicf("confirmMilestone: Transaction not found: %v", txHash)
			}

			confirmed, at := tx.GetTransaction().GetConfirmed()
			if confirmed {
				if at > milestoneIndex {
					log.Panicf("transaction %s was already confirmed by a newer milestone %d", tx.GetTransaction().GetHash(), at)
				}

				// Tx is already confirmed by another milestone => ignore
				if at < milestoneIndex {
					tx.Release() //-1
					continue
				}

				// If confirmationIndex == milestoneIndex,
				// we have to walk the ledger changes again (for re-applying the ledger changes after shutdown)
			}

			// Mark the approvees to be traversed
			txsToTraverse[tx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetTransaction().GetBranch()] = struct{}{}

			if !tx.GetTransaction().IsTail() {
				tx.Release() //-1
				continue
			}

			txBundle := tx.GetTransaction().Tx.Bundle
			tx.Release() //-1

			bundleBucket, err := tangle.GetBundleBucket(txBundle)
			if err != nil {
				log.Panicf("confirmMilestone: BundleBucket not found: %v, Error: %v", txBundle, err)
			}

			bundle := bundleBucket.GetBundleOfTailTransaction(txHash)
			if bundle == nil {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", txHash, txBundle)
			}

			if !bundle.IsComplete() {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not complete: %v", txHash, txBundle)
			}

			if !bundle.IsValid() {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not valid: %v", txHash, txBundle)
			}

			ledgerChanges, isValueSpamBundle := bundle.GetLedgerChanges()
			if !isValueSpamBundle {
				for address, change := range ledgerChanges {
					totalLedgerChanges[address] += change
				}
			}

			// we only add the tail transaction to the txsToConfirm set, in order to not
			// accidentally skip cones, in case the other transactions (non-tail) of the bundle do not
			// reference the same trunk transaction (as seen from the PoV of the bundle).
			// if we wouldn't do it like this, we have a high chance of computing an
			// inconsistent ledger state.
			txsToConfirm[txHash] = struct{}{}
		}
	}
	tc := time.Now()

	err := tangle.ApplyLedgerDiffWithoutLocking(totalLedgerChanges, milestoneIndex)
	if err != nil {
		log.Panicf("confirmMilestone: ApplyLedgerDiff failed with Error: %v", err)
	}

	for txHash := range txsToConfirm {

		cachedTx := tangle.GetCachedTransaction(txHash) //+1
		if !cachedTx.Exists() {
			log.Panicf("confirmMilestone: Transaction not found: %v", txHash)
		}

		// confirm all txs of the bundle
		bundleBucket, err := tangle.GetBundleBucket(cachedTx.GetTransaction().Tx.Bundle)
		if err != nil {
			log.Panicf("confirmMilestone: BundleBucket not found: %v, Error: %v", cachedTx.GetTransaction().Tx.Bundle, err)
		}

		// we are only iterating over tail txs
		bundle := bundleBucket.GetBundleOfTailTransaction(txHash)
		if bundle == nil {
			log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", txHash, cachedTx.GetTransaction().Tx.Bundle)
		}
		cachedTx.Release() //-1

		transactions := bundle.GetTransactions() //+1
		for _, bndlTx := range transactions {
			bndlTx.GetTransaction().SetConfirmed(true, milestoneIndex)
			Events.TransactionConfirmed.Trigger(bndlTx, milestoneIndex, milestoneTail.GetTransaction().GetTimestamp())
		}
		transactions.Release() //-1
	}

	log.Infof("Milestone confirmed (%d): txsToConfirm: %v, collect: %v, total: %v", milestoneIndex, len(txsToConfirm), tc.Sub(ts), time.Since(ts))
}
