package tangle

import (
	"github.com/iotaledger/hive.go/lru_cache"

	"github.com/gohornet/hornet/packages/profile"
)

var (
	// Transactions that approve a certain TxHash
	ApproversCache *lru_cache.LRUCache
)

func InitApproversCache() {
	opts := profile.GetProfile().Caches.Approvers
	ApproversCache = lru_cache.NewLRUCache(opts.Size, &lru_cache.LRUCacheOptions{
		EvictionCallback:  onEvictApprovers,
		EvictionBatchSize: opts.EvictionSize,
	})
}

func onEvictApprovers(_ interface{}, values interface{}) {
	valT := values.([]interface{})

	var approvers []*Approvers
	for _, obj := range valT {
		approvers = append(approvers, obj.(*Approvers))
	}

	if err := storeApproversInDatabase(approvers); err != nil {
		panic(err)
	}
}

func FlushApproversCache() {
	ApproversCache.DeleteAll()
}
