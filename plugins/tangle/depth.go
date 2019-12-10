package tangle

import (
	"math"
	"sync"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

// keeps track of whether a tx is below the max depth for the given milestone
type belowDepthMemoizationCache struct {
	mu               sync.RWMutex
	milestone        milestone_index.MilestoneIndex
	memoizationCache map[trinary.Hash]bool
	hits             uint64
	misses           uint64
}

func (bdmc *belowDepthMemoizationCache) IsBelowMaxDepth(hash trinary.Hash) *bool {
	bdmc.mu.RLock()
	is, has := bdmc.memoizationCache[hash]
	if !has {
		bdmc.misses++
		bdmc.mu.RUnlock()
		return nil
	}
	bdmc.hits++
	bdmc.mu.RUnlock()
	return &is
}

func (bdmc *belowDepthMemoizationCache) ResetIfNewerMilestone(currentIndex milestone_index.MilestoneIndex) {
	bdmc.mu.Lock()
	if currentIndex > bdmc.milestone {
		bdmc.memoizationCache = make(map[trinary.Hash]bool)
		bdmc.milestone = currentIndex
	}
	bdmc.mu.Unlock()
}

func (bdmc *belowDepthMemoizationCache) Set(hash trinary.Hash, belowMaxDepth bool) {
	bdmc.mu.Lock()
	bdmc.memoizationCache[hash] = belowMaxDepth
	bdmc.mu.Unlock()
}

func (bdmc *belowDepthMemoizationCache) CacheHitRatio() float64 {
	if bdmc.hits == 0 || bdmc.misses == 0 {
		return 0
	}
	return math.Round((float64(bdmc.hits)/float64(bdmc.hits+bdmc.misses))*100) / 100
}

// BelowDepthMemoizationCache caches hashes below a max depth
var BelowDepthMemoizationCache = belowDepthMemoizationCache{memoizationCache: make(map[trinary.Hash]bool)}

// IsBelowMaxDepth checks whether the given transaction is below the max depth by first checking whether it is confirmed
// within the range of allowed milestones. if not, it is checked whether any referenced (directly/indirectly) tx
// is confirmed by a milestone below the allowed threshold until a limit is reached of analyzed txs, in which case
// the given tail transaction is also deemed being below max depth.
func IsBelowMaxDepth(tailTx *hornet.Transaction, lowerAllowedSnapshotIndex int) bool {

	// if the tx is already confirmed we don't need to check it for max depth
	if confirmed, at := tailTx.GetConfirmed(); confirmed && int(at) >= lowerAllowedSnapshotIndex {
		return false
	}

	// no need to evaluate whether the tx is above/below the max depth if already checked
	if is := BelowDepthMemoizationCache.IsBelowMaxDepth(tailTx.GetHash()); is != nil {
		return *is
	}

	// if the transaction is unconfirmed
	txsToTraverse := make(map[string]struct{})
	txsToTraverse[tailTx.GetHash()] = struct{}{}
	analyzedTxs := make(map[string]struct{})

	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			// if we analyzed the limit we flag the tx automatically as below the max depth
			// TODO: check whether a fast tangle would hit this limit naturally
			if len(analyzedTxs) == belowMaxDepthTransactionLimit {
				BelowDepthMemoizationCache.Set(tailTx.GetHash(), true)
				return true
			}

			// don't re-analyze an already analyzed transaction
			if _, alreadyAnalyzed := analyzedTxs[txHash]; alreadyAnalyzed {
				continue
			}
			analyzedTxs[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(txHash) {
				continue
			}

			// we don't need to analyze further down if we already memoized this particular tx's max depth validity
			if is := BelowDepthMemoizationCache.IsBelowMaxDepth(txHash); is != nil {
				if *is {
					BelowDepthMemoizationCache.Set(tailTx.GetHash(), true)
					return true
				}
				continue
			}

			tx, err := tangle.GetTransaction(txHash)
			if err != nil {
				log.Panic(err)
			}

			// we should have the transaction because the to be checked tail tx is solid
			// and we passed the point where we checked whether the tx is a solid entry point
			if tx == nil {
				log.Panicf("missing transaction %s for below max depth check", txHash)
			}

			confirmed, at := tx.GetConfirmed()

			// we are below max depth on this transaction if it is confirmed by a milestone below our threshold
			if confirmed && int(at) < lowerAllowedSnapshotIndex {
				BelowDepthMemoizationCache.Set(tailTx.GetHash(), true)
				return true
			}

			// we don't need to analyze further down if the transaction is confirmed within the threshold of the max depth
			if confirmed {
				continue
			}

			//
			txsToTraverse[tx.GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetBranch()] = struct{}{}
		}
	}

	// memorize that all traversed txs are not below the max depth
	// (this also includes the start tail transaction)
	for analyzedTxHash := range analyzedTxs {
		BelowDepthMemoizationCache.Set(analyzedTxHash, false)
	}
	return false
}
