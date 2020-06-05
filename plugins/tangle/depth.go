package tangle

import (
	"math"
	"sync"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// keeps track of whether a tx is below the max depth for the given milestone
type belowDepthMemoizationCache struct {
	mu               sync.RWMutex
	milestone        milestone.Index
	memoizationCache map[string]bool
	hits             uint64
	misses           uint64
}

func (bdmc *belowDepthMemoizationCache) IsBelowMaxDepth(hash hornet.Hash) *bool {
	bdmc.mu.RLock()
	is, has := bdmc.memoizationCache[string(hash)]
	if !has {
		bdmc.misses++
		bdmc.mu.RUnlock()
		return nil
	}
	bdmc.hits++
	bdmc.mu.RUnlock()
	return &is
}

func (bdmc *belowDepthMemoizationCache) ResetIfNewerMilestone(currentIndex milestone.Index) {
	bdmc.mu.Lock()
	if currentIndex > bdmc.milestone {
		bdmc.memoizationCache = make(map[string]bool)
		bdmc.milestone = currentIndex
	}
	bdmc.mu.Unlock()
}

func (bdmc *belowDepthMemoizationCache) Set(hash hornet.Hash, belowMaxDepth bool) {
	bdmc.mu.Lock()
	bdmc.memoizationCache[string(hash)] = belowMaxDepth
	bdmc.mu.Unlock()
}

func (bdmc *belowDepthMemoizationCache) CacheHitRatio() float64 {
	if bdmc.hits == 0 || bdmc.misses == 0 {
		return 0
	}
	return math.Round((float64(bdmc.hits)/float64(bdmc.hits+bdmc.misses))*100) / 100
}

// BelowDepthMemoizationCache caches hashes below a max depth
var BelowDepthMemoizationCache = belowDepthMemoizationCache{memoizationCache: make(map[string]bool)}

// IsBelowMaxDepth checks whether the given transaction is below the max depth by first checking whether it is confirmed
// within the range of allowed milestones. if not, it is checked whether any referenced (directly/indirectly) tx
// is confirmed by a milestone below the allowed threshold until a limit is reached of analyzed txs, in which case
// the given tail transaction is also deemed being below max depth.
func IsBelowMaxDepth(cachedTailTx *tangle.CachedTransaction, lowerAllowedSnapshotIndex int, forceRelease bool) bool {

	defer cachedTailTx.Release(forceRelease) // tx -1

	// if the tx is already confirmed we don't need to check it for max depth
	if confirmed, at := cachedTailTx.GetMetadata().GetConfirmed(); confirmed && int(at) >= lowerAllowedSnapshotIndex {
		return false
	}

	// no need to evaluate whether the tx is above/below the max depth if already checked
	if is := BelowDepthMemoizationCache.IsBelowMaxDepth(cachedTailTx.GetTransaction().GetTxHash()); is != nil {
		return *is
	}

	// if the transaction is unconfirmed
	txsToTraverse := make(map[string]struct{})
	txsToTraverse[string(cachedTailTx.GetTransaction().GetTxHash())] = struct{}{}
	analyzedTxs := make(map[string]struct{})

	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			// if we analyzed the limit we flag the tx automatically as below the max depth
			// TODO: check whether a fast tangle would hit this limit naturally
			if len(analyzedTxs) == belowMaxDepthTransactionLimit {
				BelowDepthMemoizationCache.Set(cachedTailTx.GetTransaction().GetTxHash(), true)
				return true
			}

			// don't re-analyze an already analyzed transaction
			if _, alreadyAnalyzed := analyzedTxs[txHash]; alreadyAnalyzed {
				continue
			}
			analyzedTxs[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(hornet.Hash(txHash)) {
				continue
			}

			// we don't need to analyze further down if we already memoized this particular tx's max depth validity
			if is := BelowDepthMemoizationCache.IsBelowMaxDepth(hornet.Hash(txHash)); is != nil {
				if *is {
					BelowDepthMemoizationCache.Set(cachedTailTx.GetTransaction().GetTxHash(), true)
					return true
				}
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHash)) // tx +1

			// we should have the transaction because the to be checked tail tx is solid
			// and we passed the point where we checked whether the tx is a solid entry point
			if cachedTx == nil {
				log.Panicf("missing transaction %s for below max depth check", hornet.Hash(txHash).Trytes())
			}

			confirmed, at := cachedTx.GetMetadata().GetConfirmed()

			// we are below max depth on this transaction if it is confirmed by a milestone below our threshold
			if confirmed && int(at) < lowerAllowedSnapshotIndex {
				BelowDepthMemoizationCache.Set(cachedTailTx.GetTransaction().GetTxHash(), true)
				cachedTx.Release(forceRelease) // tx -1
				return true
			}

			// we don't need to analyze further down if the transaction is confirmed within the threshold of the max depth
			if confirmed {
				cachedTx.Release(forceRelease) // tx -1
				continue
			}

			//
			txsToTraverse[string(cachedTx.GetTransaction().GetTrunkHash())] = struct{}{}
			txsToTraverse[string(cachedTx.GetTransaction().GetBranchHash())] = struct{}{}
			cachedTx.Release(forceRelease) // tx -1
		}
	}

	// memorize that all traversed txs are not below the max depth
	// (this also includes the start tail transaction)
	for analyzedTxHash := range analyzedTxs {
		BelowDepthMemoizationCache.Set(hornet.Hash(analyzedTxHash), false)
	}
	return false
}
