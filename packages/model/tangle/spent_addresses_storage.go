package tangle

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/profile"
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

func spentAddressFactory(key []byte) objectstorage.StorableObject {
	sa := &hornet.SpentAddress{
		Address: make([]byte, 49),
	}
	copy(sa.Address, key[:49])
	return sa
}

func GetSpentAddressesStorageSize() int {
	return spentAddressesStorage.GetSize()
}

func configureSpentAddressesStorage() {

	opts := profile.GetProfile().Caches.SpentAddresses

	spentAddressesStorage = objectstorage.New(
		database.GetHornetBadgerInstance(),
		[]byte{DBPrefixSpentAddresses},
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

	spentAddress := spentAddressFactory(trinary.MustTrytesToBytes(address)[:49])

	newlyAdded := false
	spentAddressesStorage.ComputeIfAbsent(spentAddress.GetStorageKey(), func(key []byte) objectstorage.StorableObject {
		newlyAdded = true
		return spentAddress
	}).Release()

	return newlyAdded
}

// spentAddress +-0
func MarkAddressAsSpentBinaryWithoutLocking(address []byte) bool {

	spentAddress := spentAddressFactory(address[:49])

	newlyAdded := false
	spentAddressesStorage.ComputeIfAbsent(spentAddress.GetStorageKey(), func(key []byte) objectstorage.StorableObject {
		newlyAdded = true
		return spentAddress
	}).Release()

	return newlyAdded
}

func CountSpentAddressesEntriesWithoutLocking() (spentAddressesCount int32) {

	spentAddressesCount = 0
	spentAddressesStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		spentAddressesCount++
		cachedObject.Release() // spentAddress -1
		return true
	})

	return spentAddressesCount
}

// StreamSpentAddressesToWriter streams all spent addresses directly to an io.Writer.
// ReadLockSpentAddresses must be held while entering this function.
func StreamSpentAddressesToWriter(buf io.Writer, spentAddressesCount int32, abortSignal <-chan struct{}) error {

	var addressesWritten int32

	var err error
	wasAborted := false
	spentAddressesStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Release() // spentAddress -1

		select {
		case <-abortSignal:
			wasAborted = true
			return false
		default:
		}

		addressesWritten++
		return (binary.Write(buf, binary.LittleEndian, key) == nil)
	})

	if err != nil {
		return err
	}

	if wasAborted {
		return ErrOperationAborted
	}

	if addressesWritten != spentAddressesCount {
		return fmt.Errorf("Amount of spent addresses changed during write %d/%d", addressesWritten, spentAddressesCount)
	}

	return nil
}

func ShutdownSpentAddressesStorage() {
	spentAddressesStorage.Shutdown()
}
