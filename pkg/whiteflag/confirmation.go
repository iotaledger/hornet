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
// all cachedTxMetas have to be released outside.
func ConfirmMilestone(cachedTxMetas map[string]*tangle.CachedMetadata, cachedMsBundle *tangle.CachedBundle, forEachConfirmedTx func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64), onMilestoneConfirmed func(confirmation *Confirmation)) (*ConfirmedMilestoneStats, error) {
	defer cachedMsBundle.Release(true)
	msBundle := cachedMsBundle.GetBundle()

	cachedBundles := make(map[string]*tangle.CachedBundle)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// release all bundles at the end
		for _, cachedBundle := range cachedBundles {
			cachedBundle.Release(true) // bundle -1
		}
	}()

	if _, exists := cachedBundles[string(cachedMsBundle.GetBundle().GetTailHash())]; !exists {
		// release the bundles at the end to speed up calculation
		cachedBundles[string(cachedMsBundle.GetBundle().GetTailHash())] = cachedMsBundle.Retain()
	}

	tangle.WriteLockLedger()
	defer tangle.WriteUnlockLedger()

	milestoneIndex := msBundle.GetMilestoneIndex()

	ts := time.Now()

	mutations, err := ComputeWhiteFlagMutations(cachedTxMetas, cachedBundles, tangle.GetMilestoneMerkleHashFunc(), msBundle.GetTailHash())
	if err != nil {
		// According to the RFC we should panic if we encounter any invalid bundles during confirmation
		return nil, fmt.Errorf("confirmMilestone: whiteflag.ComputeConfirmation failed with Error: %v", err)
	}

	confirmation := &Confirmation{
		MilestoneIndex: milestoneIndex,
		MilestoneHash:  msBundle.GetTailHash(),
		Mutations:      mutations,
	}

	// Verify the calculated MerkleTreeHash with the one inside the milestone
	merkleTreeHash, err := msBundle.GetMilestoneMerkleTreeHash()
	if err != nil {
		return nil, fmt.Errorf("confirmMilestone: invalid MerkleTreeHash: %w", err)
	}
	if !bytes.Equal(mutations.MerkleTreeHash, merkleTreeHash) {
		return nil, fmt.Errorf("confirmMilestone: computed MerkleTreeHash %s does not match the value in the milestone %s", hex.EncodeToString(mutations.MerkleTreeHash), hex.EncodeToString(merkleTreeHash))
	}

	tc := time.Now()

	err = tangle.ApplyLedgerDiffWithoutLocking(mutations.AddressMutations, milestoneIndex)
	if err != nil {
		return nil, fmt.Errorf("confirmMilestone: ApplyLedgerDiff failed with Error: %v", err)
	}

	cachedMsTailTx := msBundle.GetTail()
	defer cachedMsTailTx.Release(true)

	loadTxMeta := func(txHash hornet.Hash) (*tangle.CachedMetadata, error) {
		cachedTxMeta, exists := cachedTxMetas[string(txHash)]
		if !exists {
			cachedTxMeta = tangle.GetCachedTxMetadataOrNil(txHash) // meta +1
			if cachedTxMeta == nil {
				return nil, fmt.Errorf("confirmMilestone: Transaction not found: %v", txHash.Trytes())
			}
			cachedTxMetas[string(txHash)] = cachedTxMeta
		}
		return cachedTxMeta, nil
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
	forEachBundleTxMetaWithTailTxHash := func(txHash hornet.Hash, do func(tx *tangle.CachedMetadata)) error {
		bundle, err := loadBundle(txHash)
		if err != nil {
			return err
		}
		bundleTxHashes := bundle.GetBundle().GetTxHashes()
		for _, bundleTxHash := range bundleTxHashes {
			cachedBundleTx, err := loadTxMeta(bundleTxHash)
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
	for _, txHash := range mutations.TailsIncluded {
		if err := forEachBundleTxMetaWithTailTxHash(txHash, func(txMeta *tangle.CachedMetadata) {
			if !txMeta.GetMetadata().IsConfirmed() {
				txMeta.GetMetadata().SetConfirmed(true, milestoneIndex)
				txMeta.GetMetadata().SetRootSnapshotIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.TxsConfirmed++
				conf.TxsValue++
				metrics.SharedServerMetrics.ValueTransactions.Inc()
				metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
				forEachConfirmedTx(txMeta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all txs of the zero value tails
	for _, txHash := range mutations.TailsExcludedZeroValue {
		if err := forEachBundleTxMetaWithTailTxHash(txHash, func(txMeta *tangle.CachedMetadata) {
			if !txMeta.GetMetadata().IsConfirmed() {
				txMeta.GetMetadata().SetConfirmed(true, milestoneIndex)
				txMeta.GetMetadata().SetRootSnapshotIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.TxsConfirmed++
				conf.TxsZeroValue++
				metrics.SharedServerMetrics.ZeroValueTransactions.Inc()
				metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
				forEachConfirmedTx(txMeta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	// confirm all conflicting txs of the conflicting tails
	for _, txHash := range mutations.TailsExcludedConflicting {
		if err := forEachBundleTxMetaWithTailTxHash(txHash, func(txMeta *tangle.CachedMetadata) {
			txMeta.GetMetadata().SetConflicting(true)
			if !txMeta.GetMetadata().IsConfirmed() {
				txMeta.GetMetadata().SetConfirmed(true, milestoneIndex)
				txMeta.GetMetadata().SetRootSnapshotIndexes(milestoneIndex, milestoneIndex, milestoneIndex)
				conf.TxsConfirmed++
				conf.TxsConflicting++
				metrics.SharedServerMetrics.ConflictingTransactions.Inc()
				metrics.SharedServerMetrics.ConfirmedTransactions.Inc()
				forEachConfirmedTx(txMeta, milestoneIndex, confirmationTime)
			}
		}); err != nil {
			return nil, err
		}
	}

	onMilestoneConfirmed(confirmation)

	conf.Collecting = tc.Sub(ts)
	conf.Total = time.Since(ts)

	return conf, nil
}
