package tangle

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/database"

	hornetDB "github.com/gohornet/hornet/packages/database"
)

var (
	spentAddressesDatabase                database.Database
	spentAddressesDatabaseTransactionLock sync.RWMutex
)

func ReadLockSpentAddresses() {
	spentAddressesDatabaseTransactionLock.RLock()
}

func ReadUnlockSpentAddresses() {
	spentAddressesDatabaseTransactionLock.RUnlock()
}

func WriteLockSpentAddresses() {
	spentAddressesDatabaseTransactionLock.Lock()
}

func WriteUnlockSpentAddresses() {
	spentAddressesDatabaseTransactionLock.Unlock()
}

func configureSpentAddressesDatabase() {
	if db, err := database.Get(DBPrefixSpentAddresses, hornetDB.GetBadgerInstance()); err != nil {
		panic(err)
	} else {
		spentAddressesDatabase = db
	}
}

func databaseKeyForAddress(address trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(address)
}

func spentDatabaseContainsAddress(address trinary.Hash) (bool, error) {
	ReadLockSpentAddresses()
	defer ReadUnlockSpentAddresses()

	if contains, err := spentAddressesDatabase.Contains(databaseKeyForAddress(address)); err != nil {
		return contains, errors.Wrap(NewDatabaseError(err), "failed to check if the address exists in the spent addresses database")
	} else {
		return contains, nil
	}
}

func storeSpentAddressesInDatabase(spent []trinary.Hash) error {
	WriteLockSpentAddresses()
	defer WriteUnlockSpentAddresses()

	var entries []database.Entry

	for _, address := range spent {
		key := databaseKeyForAddress(address)

		entries = append(entries, database.Entry{
			Key:   key,
			Value: []byte{},
		})
	}

	// Now batch insert/delete all entries
	if err := spentAddressesDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to mark addresses as spent")
	}

	return nil
}

func StoreSpentAddressesBytesInDatabase(spentInBytes [][]byte) error {
	WriteLockSpentAddresses()
	defer WriteUnlockSpentAddresses()

	var entries []database.Entry

	for _, addressInBytes := range spentInBytes {
		key := addressInBytes

		entries = append(entries, database.Entry{
			Key:   key,
			Value: []byte{},
		})
	}

	// Now batch insert/delete all entries
	if err := spentAddressesDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to mark addresses as spent")
	}

	return nil
}

// Addresses should be locked between CountSpentAddressesEntries and StreamSpentAddressesToWriter
func CountSpentAddressesEntries() (int32, error) {

	var addressesCount int32
	err := spentAddressesDatabase.StreamForEachKeyOnly(func(entry database.KeyOnlyEntry) error {
		addressesCount++
		return nil
	})

	if err != nil {
		return 0, errors.Wrap(NewDatabaseError(err), "failed to count spent addresses in database")
	}

	return addressesCount, nil
}

func StreamSpentAddressesToWriter(buf io.Writer, spentAddressesCount int32) error {

	var addressesWritten int32
	err := spentAddressesDatabase.StreamForEachKeyOnly(func(entry database.KeyOnlyEntry) error {
		addressesWritten++
		return binary.Write(buf, binary.BigEndian, entry.Key)
	})

	if addressesWritten != spentAddressesCount {
		return errors.Wrapf(NewDatabaseError(err), "Amount of spent addresses changed during write %d/%d", addressesWritten, spentAddressesCount)
	}

	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to stream spent addresses from database")
	}

	return nil
}
