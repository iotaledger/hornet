package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	spentAddressesCache *datastructure.LRUCache
)

func WasAddressSpentFrom(address trinary.Hash) (result bool, err error) {
	if spentAddressesCache.Contains(address) {
		result = true
	} else {
		result, err = spentDatabaseContainsAddress(address)
	}
	return
}

func MarkAddressAsSpent(address trinary.Hash) {
	spentAddressesCache.Set(address, true)
}

func InitSpentAddressesCache() {
	spentAddressesCache = datastructure.NewLRUCache(profile.GetProfile().Caches.SpentAddresses, &datastructure.LRUCacheOptions{
		EvictionCallback:  onEvictSpentAddress,
		EvictionBatchSize: 1000,
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
	spentAddressesCache.DeleteAll()
}
