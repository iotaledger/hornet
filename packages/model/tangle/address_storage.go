package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
)

var addressesStorage *objectstorage.ObjectStorage

type CachedAddress struct {
	objectstorage.CachedObject
}

type CachedAddresses []*CachedAddress

func (cachedAddresses CachedAddresses) Release() {
	for _, cachedAddress := range cachedAddresses {
		cachedAddress.Release()
	}
}

func (c *CachedAddress) GetAddress() *hornet.Address {
	return c.Get().(*hornet.Address)
}

func addressFactory(key []byte) objectstorage.StorableObject {
	address := &hornet.Address{
		Address: make([]byte, 49),
		TxHash:  make([]byte, 49),
	}
	copy(address.Address, key[:49])
	copy(address.TxHash, key[49:])
	return address
}

func GetAddressesStorageSize() int {
	return addressesStorage.GetSize()
}

func configureAddressesStorage() {

	opts := profile.GetProfile().Caches.Addresses

	addressesStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixAddresses},
		addressFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.PartitionKey(49, 49),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// address +-0
func GetTransactionHashesForAddress(address trinary.Hash, maxFind ...int) []trinary.Hash {
	var transactionHashes []trinary.Hash

	i := 0
	addressesStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		i++
		if (len(maxFind) > 0) && (i > maxFind[0]) {
			cachedObject.Release() // address -1
			return false
		}

		if !cachedObject.Exists() {
			cachedObject.Release() // address -1
			return true
		}

		transactionHashes = append(transactionHashes, cachedObject.(*CachedAddress).GetAddress().GetTransactionHash())
		cachedObject.Release() // address -1
		return true
	}, trinary.MustTrytesToBytes(address)[:49])

	return transactionHashes
}

// address +1
func StoreAddress(address trinary.Hash, txHash trinary.Hash) *CachedAddress {

	addressObj := &hornet.Address{
		Address: trinary.MustTrytesToBytes(address)[:49],
		TxHash:  trinary.MustTrytesToBytes(txHash)[:49],
	}

	return &CachedAddress{addressesStorage.Store(addressObj)}
}

// address +-0
func DeleteAddress(address trinary.Hash, txHash trinary.Hash) {
	addressesStorage.Delete(append(trinary.MustTrytesToBytes(address)[:49], trinary.MustTrytesToBytes(txHash)[:49]...))
}

func ShutdownAddressStorage() {
	addressesStorage.Shutdown()
}
