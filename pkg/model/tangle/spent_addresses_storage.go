package tangle

import (
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/pkg/database"
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

func spentAddressFactory(key []byte) (objectstorage.StorableObject, int, error) {
	sa := &hornet.SpentAddress{
		Address: make([]byte, 49),
	}
	copy(sa.Address, key[:49])
	return sa, 49, nil
}

func GetSpentAddressesStorageSize() int {
	return spentAddressesStorage.GetSize()
}

func configureSpentAddressesStorage() {

	opts := profile.LoadProfile().Caches.SpentAddresses

	spentAddressesStorage = objectstorage.New(
		database.StorageWithPrefix(DBPrefixSpentAddresses),
		spentAddressFactory,
		objectstorage.CacheTime(time.Duration(opts.CacheTimeMs)*time.Millisecond),
		objectstorage.PersistenceEnabled(true),
		objectstorage.KeysOnly(true),
		objectstorage.LeakDetectionEnabled(opts.LeakDetectionOptions.Enabled,
			objectstorage.LeakDetectionOptions{
				MaxConsumersPerObject: opts.LeakDetectionOptions.MaxConsumersPerObject,
				MaxConsumerHoldTime:   time.Duration(opts.LeakDetectionOptions.MaxConsumerHoldTimeSec) * time.Second,
			}),
	)
}

// spentAddress +-0
func WasAddressSpentFrom(address trinary.Trytes) bool {
	return spentAddressesStorage.Contains(trinary.MustTrytesToBytes(address)[:49])
}

// spentAddress +-0
func MarkAddressAsSpent(address trinary.Trytes) bool {
	spentAddressesLock.Lock()
	defer spentAddressesLock.Unlock()

	spentAddress, _, _ := spentAddressFactory(trinary.MustTrytesToBytes(address)[:49])

	newlyAdded := false
	spentAddressesStorage.ComputeIfAbsent(spentAddress.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject {
		newlyAdded = true
		spentAddress.Persist()
		spentAddress.SetModified()
		return spentAddress
	}).Release(true)

	return newlyAdded
}

// spentAddress +-0
func MarkAddressAsSpentBinaryWithoutLocking(address []byte) bool {

	spentAddress, _, _ := spentAddressFactory(address[:49])

	newlyAdded := false
	spentAddressesStorage.ComputeIfAbsent(spentAddress.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject {
		newlyAdded = true
		spentAddress.Persist()
		spentAddress.SetModified()
		return spentAddress
	}).Release(true)

	return newlyAdded
}

// StreamSpentAddressesToWriter streams all spent addresses directly to an io.Writer.
func StreamSpentAddressesToWriter(buf io.Writer, abortSignal <-chan struct{}) (int32, error) {

	ReadLockSpentAddresses()
	defer ReadUnlockSpentAddresses()

	var addressesWritten int32

	wasAborted := false
	spentAddressesStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Release(true) // spentAddress -1

		select {
		case <-abortSignal:
			wasAborted = true
			return false
		default:
		}

		addressesWritten++
		return binary.Write(buf, binary.LittleEndian, key) == nil
	})

	if wasAborted {
		return 0, ErrOperationAborted
	}

	return addressesWritten, nil
}

func ShutdownSpentAddressesStorage() {
	spentAddressesStorage.Shutdown()
}
