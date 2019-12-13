package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/profile"
)

var (
	// Transactions that approve a certain TxHash
	ApproversCache *datastructure.LRUCache
)

func InitApproversCache() {
	opts := profile.GetProfile().Caches.Approvers
	ApproversCache = datastructure.NewLRUCache(opts.Size, &datastructure.LRUCacheOptions{
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
