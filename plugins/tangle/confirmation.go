package tangle

import (
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// confirmMilestone traverses a milestone and collects all unconfirmed tx,
// then the ledger diffs are calculated, the ledger state is checked and all tx are marked as confirmed.
func confirmMilestone(milestoneIndex milestone.Index, cachedMsBundle *tangle.CachedBundle) {
	defer cachedMsBundle.Release()

	ts := time.Now()

	confirmation, err := whiteflag.ComputeConfirmation(cachedMsBundle.Retain())
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid bundles during confirmation
		log.Panicf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %v", err)
	}

	tc := time.Now()

	err = tangle.ApplyLedgerDiffWithoutLocking(confirmation.AddressMutations, milestoneIndex)
	if err != nil {
		log.Panicf("confirmMilestone: ApplyLedgerDiff failed with Error: %v", err)
	}

	cachedMsTailTx := cachedMsBundle.GetBundle().GetTail()
	defer cachedMsTailTx.Release()

	cachedTxs := make(map[string]*tangle.CachedTransaction)
	cachedTxs[string(cachedMsTailTx.GetTransaction().GetTxHash())] = cachedMsTailTx
	cachedBundles := make(map[string]*tangle.CachedBundle)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// Release all bundles at the end
		for _, cachedBundle := range cachedBundles {
			cachedBundle.Release(true) // bundle -1
		}

		// Release all txs at the end
		for _, cachedTx := range cachedTxs {
			cachedTx.Release(true) // tx -1
		}
	}()

	var txsConfirmed int
	for _, txHash := range confirmation.TailsIncluded {

		cachedTx, exists := cachedTxs[string(txHash)]
		if !exists {
			cachedTx = tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				log.Panicf("confirmMilestone: Transaction not found: %v", txHash.Trytes())
			}
			cachedTxs[string(txHash)] = cachedTx
		}

		// confirm all txs of the bundle
		// we are only iterating over tail txs
		cachedBundle, exists := cachedBundles[string(txHash)]
		if !exists {
			cachedBundle = tangle.GetCachedBundleOrNil(hornet.Hash(txHash)) // bundle +1
			if cachedBundle == nil {
				//noinspection GoNilness
				log.Panicf("confirmMilestone: Tx: %v, Bundle not found: %v", txHash.Trytes(), cachedTx.GetTransaction().Tx.Bundle)
			}
			cachedBundles[string(txHash)] = cachedBundle
		}

		//noinspection GoNilness
		bundleTxHashes := cachedBundle.GetBundle().GetTxHashes()
		for _, bundleTxHash := range bundleTxHashes {

			cachedBundleTx, exists := cachedTxs[string(bundleTxHash)]
			if !exists {
				cachedBundleTx = tangle.GetCachedTransactionOrNil(bundleTxHash) // tx +1
				if cachedTx == nil {
					log.Panicf("confirmMilestone: Transaction not found: %v", bundleTxHash.Trytes())
				}
				cachedTxs[string(bundleTxHash)] = cachedBundleTx
			}

			cachedBundleTx.GetMetadata().SetConfirmed(true, milestoneIndex)
			txsConfirmed++
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			Events.TransactionConfirmed.Trigger(cachedBundleTx, milestoneIndex, cachedMsTailTx.GetTransaction().GetTimestamp())
		}
	}

	log.Infof("Milestone confirmed (%d): txsConfirmed: %v, tailsIncluded: %v, tailsConflicting: %v, tailsZeroValue: %v, collect: %v, total: %v", milestoneIndex, txsConfirmed, len(confirmation.TailsIncluded), len(confirmation.TailsExcludedConflicting), len(confirmation.TailsExcludedZeroValue), tc.Sub(ts), time.Since(ts))
}
