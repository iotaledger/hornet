package tangle

import (
	"time"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

// confirmMilestone traverses a milestone and collects all unconfirmed tx,
// then the ledger diffs are calculated, the ledger state is checked and all tx are marked as confirmed.
func confirmMilestone(milestoneIndex milestone_index.MilestoneIndex, cachedMsTailTx *tangle.CachedTransaction) {

	defer cachedMsTailTx.Release()
	ts := time.Now()

	txsToConfirm := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	totalLedgerChanges := make(map[string]int64)
	txsToTraverse[cachedMsTailTx.GetTransaction().GetHash()] = struct{}{}

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

			cachedTx := tangle.GetCachedTransaction(txHash) // tx +1
			if !cachedTx.Exists() {
				log.Panicf("confirmMilestone: Transaction not found: %v", txHash)
			}

			confirmed, at := cachedTx.GetTransaction().GetConfirmed()
			if confirmed {
				if at > milestoneIndex {
					log.Panicf("transaction %s was already confirmed by a newer milestone %d", cachedTx.GetTransaction().GetHash(), at)
				}

				// Tx is already confirmed by another milestone => ignore
				if at < milestoneIndex {
					cachedTx.Release() // tx -1
					continue
				}

				// If confirmationIndex == milestoneIndex,
				// we have to walk the ledger changes again (for re-applying the ledger changes after shutdown)
			}

			// Mark the approvees to be traversed
			txsToTraverse[cachedTx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[cachedTx.GetTransaction().GetBranch()] = struct{}{}

			if !cachedTx.GetTransaction().IsTail() {
				cachedTx.Release() // tx -1
				continue
			}

			txBundle := cachedTx.GetTransaction().Tx.Bundle
			cachedTx.Release() // tx -1

			cachedBndl := tangle.GetBundleOfTailTransaction(txHash) // bundle +1
			if cachedBndl == nil {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", txHash, txBundle)
			}

			if !cachedBndl.GetBundle().IsValid() {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not valid: %v", txHash, txBundle)
			}

			if !cachedBndl.GetBundle().IsValueSpam() {
				ledgerChanges := cachedBndl.GetBundle().GetLedgerChanges()
				for address, change := range ledgerChanges {
					totalLedgerChanges[address] += change
				}
			}

			cachedBndl.Release()

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

		cachedTx := tangle.GetCachedTransaction(txHash) // tx +1
		if !cachedTx.Exists() {
			log.Panicf("confirmMilestone: Transaction not found: %v", txHash)
		}

		// confirm all txs of the bundle
		// we are only iterating over tail txs
		cachedBndl := tangle.GetBundleOfTailTransaction(txHash) // bundle +1
		if cachedBndl == nil {
			log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", txHash, cachedTx.GetTransaction().Tx.Bundle)
		}
		cachedTx.Release() // tx -1

		cachedTxs := cachedBndl.GetBundle().GetTransactions() // txs +1
		for _, cachedBndlTx := range cachedTxs {
			cachedBndlTx.GetTransaction().SetConfirmed(true, milestoneIndex)
			Events.TransactionConfirmed.Trigger(cachedBndlTx, milestoneIndex, cachedMsTailTx.GetTransaction().GetTimestamp())
		}
		cachedTxs.Release()  // txs -1
		cachedBndl.Release() //bundle -1
	}

	log.Infof("Milestone confirmed (%d): txsToConfirm: %v, collect: %v, total: %v", milestoneIndex, len(txsToConfirm), tc.Sub(ts), time.Since(ts))
}
