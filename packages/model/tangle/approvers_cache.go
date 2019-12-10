package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
)

var (
	// Transactions that approve a certain TxHash
	approversCache *datastructure.LRUCache
)

func InitApproversCache() {
	approversCache = datastructure.NewLRUCache(ApproversCacheSize, &datastructure.LRUCacheOptions{
		EvictionCallback:  onEvictApprovers,
		EvictionBatchSize: 1000,
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
	approversCache.DeleteAll()
}
