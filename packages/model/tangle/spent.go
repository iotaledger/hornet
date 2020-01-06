package tangle

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/lru_cache"

	"github.com/gohornet/hornet/packages/profile"
)

var (
	SpentAddressesCache *lru_cache.LRUCache
)

func WasAddressSpentFrom(address trinary.Hash) (result bool, err error) {
	if SpentAddressesCache.Contains(address) {
		result = true
	} else {
		result, err = spentDatabaseContainsAddress(address)
	}
	return
}

func MarkAddressAsSpent(address trinary.Hash) {
	SpentAddressesCache.Set(address, true)
}

func InitSpentAddressesCache() {
	opts := profile.GetProfile().Caches.SpentAddresses
	SpentAddressesCache = lru_cache.NewLRUCache(opts.Size, &lru_cache.LRUCacheOptions{
		EvictionCallback:  onEvictSpentAddress,
		EvictionBatchSize: opts.EvictionSize,
	})
}

func onEvictSpentAddress(keys interface{}, _ interface{}) {
	keyT := keys.([]interface{})

	var addresses []trinary.Hash
	for _, obj := range keyT {
		addresses = append(addresses, obj.(trinary.Hash))
	}

	err := storeSpentAddressesInDatabase(addresses)
	if err != nil {
		panic(err)
	}
}

func FlushSpentAddressesCache() {
	SpentAddressesCache.DeleteAll()
}
