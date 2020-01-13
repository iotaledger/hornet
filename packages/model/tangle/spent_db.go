package tangle

import (
	"sync"

	"github.com/pkg/errors"
	cuckoo "github.com/seiflotfy/cuckoofilter"

	"github.com/iotaledger/hive.go/database"

	hornetDB "github.com/gohornet/hornet/packages/database"
)

var (
	spentAddressesDatabase database.Database
	spentAddressesLock     sync.RWMutex
	cfKey                  = []byte("cf_3")
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

func configureSpentAddressesDatabase() {
	if db, err := database.Get(DBPrefixSpentAddresses, hornetDB.GetBadgerInstance()); err != nil {
		panic(err)
	} else {
		spentAddressesDatabase = db
	}
}

func loadSpentAddressesCuckooFilter() *cuckoo.Filter {
	entry, err := spentAddressesDatabase.Get(cfKey)
	switch err {
	case database.ErrKeyNotFound:
		return cuckoo.NewFilter(CuckooFilterSize)
	case nil:
		cf, err := cuckoo.Decode(entry.Value)
		if err != nil {
			panic(err)
		}
		return cf
	default:
		panic(err)
	}
}

// Serializes the cuckoo filter holding the spent addresses to its byte representation.
// The spent addresses lock should be held while calling this function.
func SerializedSpentAddressesCuckooFilter() []byte {
	return SpentAddressesCuckooFilter.Encode()
}

// Imports the given cuckoo filter into the database.
func ImportSpentAddressesCuckooFilter(cf *cuckoo.Filter) error {
	spentAddressesLock.Lock()
	defer spentAddressesLock.Unlock()
	if err := spentAddressesDatabase.Set(database.Entry{Key: cfKey, Value: cf.Encode()}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to import spent addresses cuckoo filter")
	}
	return nil
}

// Stores the package local cuckoo filter in the database.
func StoreSpentAddressesCuckooFilterInDatabase() error {
	spentAddressesLock.Lock()
	defer spentAddressesLock.Unlock()
	if err := spentAddressesDatabase.Set(database.Entry{
		Key:   cfKey,
		Value: SpentAddressesCuckooFilter.Encode(),
	}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store spent addresses cuckoo filter")
	}
	return nil
}

// CountSpentAddressesEntries returns the amount of spent addresses.
// ReadLockSpentAddresses must be held while entering this function.
func CountSpentAddressesEntries() int32 {
	spentAddressesLock.RLock()
	defer spentAddressesLock.RUnlock()
	return int32(SpentAddressesCuckooFilter.Count())
}
