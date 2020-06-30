package tangle

import (
	"bytes"
	"encoding/hex"
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
	confirmation, err := whiteflag.ComputeConfirmation(milestoneMerkleTreeHashFunc, cachedMsBundle.Retain())
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid bundles during confirmation
		log.Panicf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %v", err)
	}

	// Verify the calculated MerkleTreeHash with the one inside the milestone
	merkleTreeHash := cachedMsBundle.GetBundle().GetMilestoneMerkleTreeHash()
	if !bytes.Equal(confirmation.MerkleTreeHash, merkleTreeHash) {
		log.Panicf("confirmMilestone: computed MerkleTreeHash %s does not match the value in the milestone %s", hex.EncodeToString(confirmation.MerkleTreeHash), hex.EncodeToString(merkleTreeHash))
	}

	tc := time.Now()

	err = tangle.ApplyLedgerDiffWithoutLocking(confirmation.AddressMutations, milestoneIndex)
	if err != nil {
		log.Panicf("confirmMilestone: ApplyLedgerDiff failed with Error: %v", err)
	}

	cachedMsTailTx := cachedMsBundle.GetBundle().GetTail()

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

	loadTx := func(txHash hornet.Hash) *tangle.CachedTransaction {
		cachedTx, exists := cachedTxs[string(txHash)]
		if !exists {
			cachedTx = tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				log.Panicf("confirmMilestone: Transaction not found: %v", txHash.Trytes())
			}
			cachedTxs[string(txHash)] = cachedTx
		}
		return cachedTx
	}

	loadBundle := func(txHash hornet.Hash) *tangle.CachedBundle {
		cachedBundle, exists := cachedBundles[string(txHash)]
		if !exists {
			cachedBundle = tangle.GetCachedBundleOrNil(txHash) // bundle +1
			if cachedBundle == nil {
				log.Panicf("confirmMilestone: Tx: %v, Bundle not found", txHash.Trytes())
			}
			cachedBundles[string(txHash)] = cachedBundle
		}
		return cachedBundle
	}

	// load the bundle for the given tail tx and iterate over each tx in the bundle
	forEachBundleTxWithTailTxHash := func(txHash hornet.Hash, do func(tx *tangle.CachedTransaction)) {
		bundleTxHashes := loadBundle(txHash).GetBundle().GetTxHashes()
		for _, bundleTxHash := range bundleTxHashes {
			cachedBundleTx := loadTx(bundleTxHash)
			do(cachedBundleTx)
		}
	}

	var txsConfirmed int
	var txsConflicting int
	var txsValue int
	var txsZeroValue int

	// confirm all txs of the included tails
	for _, txHash := range confirmation.TailsIncluded {
		forEachBundleTxWithTailTxHash(txHash, func(tx *tangle.CachedTransaction) {
			tx.GetMetadata().SetConfirmed(true, milestoneIndex)
			txsConfirmed++
			txsValue++
			metrics.SharedServerMetrics.ValueTransactions.Inc()
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			Events.TransactionConfirmed.Trigger(tx, milestoneIndex, cachedMsTailTx.GetTransaction().GetTimestamp())
		})
	}

	// confirm all txs of the zero value tails
	for _, txHash := range confirmation.TailsExcludedZeroValue {
		forEachBundleTxWithTailTxHash(txHash, func(tx *tangle.CachedTransaction) {
			tx.GetMetadata().SetConfirmed(true, milestoneIndex)
			txsConfirmed++
			txsZeroValue++
			metrics.SharedServerMetrics.ZeroValueTransactions.Inc()
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			Events.TransactionConfirmed.Trigger(tx, milestoneIndex, cachedMsTailTx.GetTransaction().GetTimestamp())
		})
	}

	// confirm all conflicting txs of the conflicting tails
	for _, txHash := range confirmation.TailsExcludedConflicting {
		forEachBundleTxWithTailTxHash(txHash, func(tx *tangle.CachedTransaction) {
			tx.GetMetadata().SetConflicting(true)
			tx.GetMetadata().SetConfirmed(true, milestoneIndex)
			txsConflicting++
			txsConfirmed++
			metrics.SharedServerMetrics.ConflictingTransactions.Inc()
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			Events.TransactionConfirmed.Trigger(tx, milestoneIndex, cachedMsTailTx.GetTransaction().GetTimestamp())
		})
	}

	log.Infof("Milestone confirmed (%d): txsConfirmed: %v, txsValue: %v, txsZeroValue: %v, txsConflicting: %v, collect: %v, total: %v", milestoneIndex, txsConfirmed, txsValue, txsZeroValue, txsConflicting, tc.Sub(ts), time.Since(ts))
}
