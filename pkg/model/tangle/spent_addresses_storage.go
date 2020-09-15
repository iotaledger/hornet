package tangle

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	spentAddressesStorage *objectstorage.ObjectStorage
	spentAddressesLock    sync.RWMutex
)

func ReadLockSpentAddresses() {
	spentAddressesLock.RLock()
}

func ReadUnlockSpentAddresses() {
	spentAddressesLock.RUnlock()
}

func WriteLockSpentAddresses() {
	spentAddressesLock.Lock()
}

func WriteUnlockSpentAddresses() {
	spentAddressesLock.Unlock()
}

type CachedSpentAddress struct {
	objectstorage.CachedObject
}

func (c *CachedSpentAddress) GetSpentAddress() *hornet.SpentAddress {
	return c.Get().(*hornet.SpentAddress)
}

func spentAddressFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	sa := hornet.NewSpentAddress(key[:49])
	return sa, nil
}

func GetSpentAddressesStorageSize() int {
	return spentAddressesStorage.GetSize()
}

func configureSpentAddressesStorage(store kvstore.KVStore, opts profile.CacheOpts) {

	spentAddressesStorage = objectstorage.New(
		store.WithRealm([]byte{StorePrefixSpentAddresses}),
		spentAddressFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.KeysOnly(true),
		objectstorage.StoreOnCreation(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// spentAddress +-0
func WasAddressSpentFrom(address hornet.Hash) bool {
	return spentAddressesStorage.Contains(address)
}

// spentAddress +-0
func MarkAddressAsSpent(address hornet.Hash) bool {
	spentAddressesLock.Lock()
	defer spentAddressesLock.Unlock()

	return MarkAddressAsSpentWithoutLocking(address)
}

// spentAddress +-0
func MarkAddressAsSpentWithoutLocking(address hornet.Hash) bool {

	spentAddress := hornet.NewSpentAddress(address[:49])

	cachedSpentAddress, newlyAdded := spentAddressesStorage.StoreIfAbsent(spentAddress)
	if cachedSpentAddress != nil {
		cachedSpentAddress.Release(true)
	}

	return newlyAdded
}

// StreamSpentAddressesToWriter streams all spent addresses directly to an io.Writer.
func StreamSpentAddressesToWriter(buf io.Writer, abortSignal <-chan struct{}) (int32, error) {

	ReadLockSpentAddresses()
	defer ReadUnlockSpentAddresses()

	var addressesWritten int32

	wasAborted := false
	spentAddressesStorage.ForEachKeyOnly(func(key []byte) bool {
		select {
		case <-abortSignal:
			wasAborted = true
			return false
		default:
		}

		addressesWritten++
		return binary.Write(buf, binary.LittleEndian, key) == nil
	}, false)

	if wasAborted {
		return 0, ErrOperationAborted
	}

	return addressesWritten, nil
}

func ShutdownSpentAddressesStorage() {
	spentAddressesStorage.Shutdown()
}

func FlushSpentAddressesStorage() {
	spentAddressesStorage.Flush()
}
