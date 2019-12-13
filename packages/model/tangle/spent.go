package tangle

import (
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	SpentAddressesCache *datastructure.LRUCache
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
	SpentAddressesCache = datastructure.NewLRUCache(opts.Size, &datastructure.LRUCacheOptions{
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
