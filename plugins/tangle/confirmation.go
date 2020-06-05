package tangle

import (
	"time"

	"github.com/gohornet/hornet/pkg/metrics"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// confirmMilestone traverses a milestone and collects all unconfirmed tx,
// then the ledger diffs are calculated, the ledger state is checked and all tx are marked as confirmed.
func confirmMilestone(milestoneIndex milestone.Index, cachedMsTailTx *tangle.CachedTransaction) {

	ts := time.Now()

	cachedTxs := make(map[string]*tangle.CachedTransaction)
	cachedTxs[string(cachedMsTailTx.GetTransaction().GetTxHash())] = cachedMsTailTx

	cachedBndls := make(map[string]*tangle.CachedBundle)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// Release all bundles at the end
		for _, cachedBndl := range cachedBndls {
			cachedBndl.Release(true) // bundle -1
		}

		// Release all txs at the end
		for _, cachedTx := range cachedTxs {
			cachedTx.Release(true) // tx -1
		}
	}()

	txsToConfirm := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	totalLedgerChanges := make(map[string]int64)
	txsToTraverse[string(cachedMsTailTx.GetTransaction().GetTxHash())] = struct{}{}

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsToConfirm[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}

			if tangle.SolidEntryPointsContain(hornet.Hash(txHash)) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx, exists := cachedTxs[txHash]
			if !exists {
				cachedTx = tangle.GetCachedTransactionOrNil(hornet.Hash(txHash)) // tx +1
				if cachedTx == nil {
					log.Panicf("confirmMilestone: Transaction not found: %v", hornet.Hash(txHash).Trytes())
				}
				cachedTxs[txHash] = cachedTx
			}

			confirmed, at := cachedTx.GetMetadata().GetConfirmed()
			if confirmed {
				if at > milestoneIndex {
					log.Panicf("transaction %s was already confirmed by a newer milestone %d", hornet.Hash(txHash).Trytes(), at)
				}

				// Tx is already confirmed by another milestone => ignore
				if at < milestoneIndex {
					continue
				}

				// If confirmationIndex == milestoneIndex,
				// we have to walk the ledger changes again (for re-applying the ledger changes after shutdown)
			}

			// Mark the approvees to be traversed
			txsToTraverse[string(cachedTx.GetTransaction().GetTrunkHash())] = struct{}{}
			txsToTraverse[string(cachedTx.GetTransaction().GetBranchHash())] = struct{}{}

			if !cachedTx.GetTransaction().IsTail() {
				continue
			}

			txBundle := cachedTx.GetTransaction().Tx.Bundle

			cachedBndl, exists := cachedBndls[txHash]
			if !exists {
				cachedBndl = tangle.GetCachedBundleOrNil(hornet.Hash(txHash)) // bundle +1
				if cachedBndl == nil {
					log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", hornet.Hash(txHash).Trytes(), txBundle)
				}
				cachedBndls[txHash] = cachedBndl
			}

			if !cachedBndl.GetBundle().IsValid() {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not valid: %v", hornet.Hash(txHash).Trytes(), txBundle)
			}

			if !cachedBndl.GetBundle().IsValueSpam() {
				ledgerChanges := cachedBndl.GetBundle().GetLedgerChanges()
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

		cachedTx, exists := cachedTxs[txHash]
		if !exists {
			cachedTx = tangle.GetCachedTransactionOrNil(hornet.Hash(txHash)) // tx +1
			if cachedTx == nil {
				log.Panicf("confirmMilestone: Transaction not found: %v", hornet.Hash(txHash).Trytes())
			}
			cachedTxs[txHash] = cachedTx
		}

		// confirm all txs of the bundle
		// we are only iterating over tail txs
		cachedBndl, exists := cachedBndls[txHash]
		if !exists {
			cachedBndl = tangle.GetCachedBundleOrNil(hornet.Hash(txHash)) // bundle +1
			if cachedBndl == nil {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", hornet.Hash(txHash).Trytes(), cachedTx.GetTransaction().Tx.Bundle)
			}
			cachedBndls[txHash] = cachedBndl
		}

		bndlTxHashes := cachedBndl.GetBundle().GetTxHashes()
		for _, bndlTxHash := range bndlTxHashes {

			cachedBndlTx, exists := cachedTxs[string(bndlTxHash)]
			if !exists {
				cachedBndlTx = tangle.GetCachedTransactionOrNil(bndlTxHash) // tx +1
				if cachedTx == nil {
					log.Panicf("confirmMilestone: Transaction not found: %v", bndlTxHash.Trytes())
				}
				cachedTxs[string(bndlTxHash)] = cachedBndlTx
			}

			cachedBndlTx.GetMetadata().SetConfirmed(true, milestoneIndex)
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			Events.TransactionConfirmed.Trigger(cachedBndlTx, milestoneIndex, cachedMsTailTx.GetTransaction().GetTimestamp())
		}
	}

	log.Infof("Milestone confirmed (%d): txsToConfirm: %v, collect: %v, total: %v", milestoneIndex, len(txsToConfirm), tc.Sub(ts), time.Since(ts))
}
