package whiteflag

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

type ConfirmedMilestoneStats struct {
	Index            milestone.Index
	ConfirmationTime int64
	Txs              tangle.CachedTransactions
	TxsConfirmed     int
	TxsConflicting   int
	TxsValue         int
	TxsZeroValue     int
	Collecting       time.Duration
	Total            time.Duration
}

// ConfirmMilestone traverses a milestone and collects all unconfirmed tx,
// then the ledger diffs are calculated, the ledger state is checked and all tx are marked as confirmed.
func ConfirmMilestone(cachedMsBundle *tangle.CachedBundle, forEachConfirmedTx func(tx *tangle.CachedTransaction, index milestone.Index, confTime int64)) (*ConfirmedMilestoneStats, error) {
	defer cachedMsBundle.Release()

	tangle.WriteLockLedger()
	defer tangle.WriteUnlockLedger()

	milestoneIndex := cachedMsBundle.GetBundle().GetMilestoneIndex()

	ts := time.Now()
	confirmation, err := ComputeConfirmation(tangle.GetMilestoneMerkleHashFunc(), cachedMsBundle.Retain())
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid bundles during confirmation
		return nil, fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %v", err)
	}

	// Verify the calculated MerkleTreeHash with the one inside the milestone
	merkleTreeHash := cachedMsBundle.GetBundle().GetMilestoneMerkleTreeHash()
	if !bytes.Equal(confirmation.MerkleTreeHash, merkleTreeHash) {
		return nil, fmt.Errorf("confirmMilestone: computed MerkleTreeHash %s does not match the value in the milestone %s", hex.EncodeToString(confirmation.MerkleTreeHash), hex.EncodeToString(merkleTreeHash))
	}

	tc := time.Now()

	err = tangle.ApplyLedgerDiffWithoutLocking(confirmation.AddressMutations, milestoneIndex)
	if err != nil {
		return nil, fmt.Errorf("confirmMilestone: ApplyLedgerDiff failed with Error: %v", err)
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

	loadTx := func(txHash hornet.Hash) (*tangle.CachedTransaction, error) {
		cachedTx, exists := cachedTxs[string(txHash)]
		if !exists {
			cachedTx = tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				return nil, fmt.Errorf("confirmMilestone: Transaction not found: %v", txHash.Trytes())
			}
			cachedTxs[string(txHash)] = cachedTx
		}
		return cachedTx, nil
	}

	loadBundle := func(txHash hornet.Hash) (*tangle.CachedBundle, error) {
		cachedBundle, exists := cachedBundles[string(txHash)]
		if !exists {
			cachedBundle = tangle.GetCachedBundleOrNil(txHash) // bundle +1
			if cachedBundle == nil {
				return nil, fmt.Errorf("confirmMilestone: Tx: %v, Bundle not found", txHash.Trytes())
			}
			cachedBundles[string(txHash)] = cachedBundle
		}
		return cachedBundle, nil
	}

	// load the bundle for the given tail tx and iterate over each tx in the bundle
	forEachBundleTxWithTailTxHash := func(txHash hornet.Hash, do func(tx *tangle.CachedTransaction)) error {
		bundle, err := loadBundle(txHash)
		if err != nil {
			return err
		}
		bundleTxHashes := bundle.GetBundle().GetTxHashes()
		for _, bundleTxHash := range bundleTxHashes {
			cachedBundleTx, err := loadTx(bundleTxHash)
			if err != nil {
				return err
			}
			do(cachedBundleTx)
		}
		return nil
	}

	conf := &ConfirmedMilestoneStats{
		Index: milestoneIndex,
	}

	confirmationTime := cachedMsTailTx.GetTransaction().GetTimestamp()

	// confirm all txs of the included tails
	for _, txHash := range confirmation.TailsIncluded {
		if err := forEachBundleTxWithTailTxHash(txHash, func(tx *tangle.CachedTransaction) {
			tx.GetMetadata().SetConfirmed(true, milestoneIndex)
			conf.TxsConfirmed++
			conf.TxsValue++
			metrics.SharedServerMetrics.ValueTransactions.Inc()
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			forEachConfirmedTx(tx, milestoneIndex, confirmationTime)
		}); err != nil {
			return nil, err
		}
	}

	// confirm all txs of the zero value tails
	for _, txHash := range confirmation.TailsExcludedZeroValue {
		if err := forEachBundleTxWithTailTxHash(txHash, func(tx *tangle.CachedTransaction) {
			tx.GetMetadata().SetConfirmed(true, milestoneIndex)
			conf.TxsConfirmed++
			conf.TxsZeroValue++
			metrics.SharedServerMetrics.ZeroValueTransactions.Inc()
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			forEachConfirmedTx(tx, milestoneIndex, confirmationTime)
		}); err != nil {
			return nil, err
		}
	}

	// confirm all conflicting txs of the conflicting tails
	for _, txHash := range confirmation.TailsExcludedConflicting {
		if err := forEachBundleTxWithTailTxHash(txHash, func(tx *tangle.CachedTransaction) {
			tx.GetMetadata().SetConflicting(true)
			tx.GetMetadata().SetConfirmed(true, milestoneIndex)
			conf.TxsConfirmed++
			conf.TxsConflicting++
			metrics.SharedServerMetrics.ConflictingTransactions.Inc()
			metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
			forEachConfirmedTx(tx, milestoneIndex, confirmationTime)
		}); err != nil {
			return nil, err
		}
	}

	conf.Collecting = tc.Sub(ts)
	conf.Total = time.Since(ts)

	return conf, nil
}
