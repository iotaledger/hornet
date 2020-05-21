package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

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

func databaseKeyPrefixForAddress(address trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(address)[:49]
}

func databaseKeyPrefixForAddressTransaction(address trinary.Hash, txHash trinary.Hash, isValue bool) []byte {
	var isValueByte byte
	if isValue {
		isValueByte = hornet.AddressTxIsValue
	}

	result := append(databaseKeyPrefixForAddress(address), isValueByte)
	return append(result, trinary.MustTrytesToBytes(txHash)[:49]...)
}

func addressFactory(key []byte) (objectstorage.StorableObject, int, error) {
	address := &hornet.Address{
		Address: make([]byte, 49),
		IsValue: key[49] == hornet.AddressTxIsValue,
		TxHash:  make([]byte, 49),
	}
	copy(address.Address, key[:49])
	copy(address.TxHash, key[50:])
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
func GetTransactionHashesForAddress(address trinary.Hash, valueOnly bool, forceRelease bool, maxFind ...int) []trinary.Hash {

	searchPrefix := databaseKeyPrefixForAddress(address)
	if valueOnly {
		var isValueByte byte = hornet.AddressTxIsValue
		searchPrefix = append(searchPrefix, isValueByte)
	}

	var transactionHashes []trinary.Hash

	i := 0
	addressesStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release(true) // address -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release(true) // address -1
			return true
		}

		transactionHashes = append(transactionHashes, (&CachedAddress{CachedObject: cachedObject}).GetAddress().GetTransactionHash())
		cachedObject.Release(forceRelease) // address -1
		return true
	}, searchPrefix)

	return transactionHashes
}

// address +1
func StoreAddress(address trinary.Hash, txHash trinary.Hash, isValue bool) *CachedAddress {

	addressObj := &hornet.Address{
		Address: trinary.MustTrytesToBytes(address)[:49],
		IsValue: isValue,
		TxHash:  trinary.MustTrytesToBytes(txHash)[:49],
	}

	cachedObj := addressesStorage.ComputeIfAbsent(addressObj.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject { // address +1
		addressObj.Persist()
		addressObj.SetModified()
		return addressObj
	})

	return &CachedAddress{CachedObject: cachedObj}
}

// address +-0
func DeleteAddress(address trinary.Hash, txHash trinary.Hash) {
	addressesStorage.Delete(databaseKeyPrefixForAddressTransaction(address, txHash, false))
	addressesStorage.Delete(databaseKeyPrefixForAddressTransaction(address, txHash, true))
}

func ShutdownAddressStorage() {
	addressesStorage.Shutdown()
}

func FlushAddressStorage() {
	addressesStorage.Flush()
}
