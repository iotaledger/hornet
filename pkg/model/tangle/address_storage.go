package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

var addressesStorage *objectstorage.ObjectStorage

type CachedAddress struct {
	objectstorage.CachedObject
}

type CachedAddresses []*CachedAddress

func (cachedAddresses CachedAddresses) Release(force ...bool) {
	for _, cachedAddress := range cachedAddresses {
		cachedAddress.Release(force...)
	}
}

func (c *CachedAddress) GetAddress() *hornet.Address {
	return c.Get().(*hornet.Address)
}

func databaseKeyPrefixForAddress(address hornet.Hash) []byte {
	return address
}

func databaseKeyPrefixForAddressTransaction(address hornet.Hash, txHash hornet.Hash, isValue bool) []byte {
	var isValueByte byte
	if isValue {
		isValueByte = hornet.AddressTxIsValue
	}

	result := append(databaseKeyPrefixForAddress(address), isValueByte)
	return append(result, txHash...)
}

func addressFactory(key []byte) (objectstorage.StorableObject, int, error) {
	address := hornet.NewAddress(key[:49], key[50:99], key[49] == hornet.AddressTxIsValue)
	return address, 99, nil
}

func GetAddressesStorageSize() int {
	return addressesStorage.GetSize()
}

func configureAddressesStorage(store kvstore.KVStore) {

	opts := profile.LoadProfile().Caches.Addresses

	addressesStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixAddresses}),
		addressFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 1, 49),
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// address +-0
func GetTransactionHashesForAddress(address hornet.Hash, valueOnly bool, forceRelease bool, maxFind ...int) hornet.Hashes {

	searchPrefix := databaseKeyPrefixForAddress(address)
	if valueOnly {
		var isValueByte byte = hornet.AddressTxIsValue
		searchPrefix = append(searchPrefix, isValueByte)
	}

	var txHashes hornet.Hashes

	i := 0
	addressesStorage.ForEachKeyOnly(func(key []byte) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			return false
		}

		txHashes = append(txHashes, key[50:99])
		return true
	}, false, searchPrefix)

	return txHashes
}

// address +1
func StoreAddress(address hornet.Hash, txHash hornet.Hash, isValue bool) *CachedAddress {

	addressObj := hornet.NewAddress(address, txHash, isValue)

	cachedObj := addressesStorage.ComputeIfAbsent(addressObj.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // address +1
		addressObj.Persist()
		addressObj.SetModified()
		return addressObj
	})

	return &CachedAddress{CachedObject: cachedObj}
}

// address +-0
func DeleteAddress(address hornet.Hash, txHash hornet.Hash) {
	addressesStorage.Delete(databaseKeyPrefixForAddressTransaction(address, txHash, false))
	addressesStorage.Delete(databaseKeyPrefixForAddressTransaction(address, txHash, true))
}

func ShutdownAddressStorage() {
	addressesStorage.Shutdown()
}

func FlushAddressStorage() {
	addressesStorage.Flush()
}
