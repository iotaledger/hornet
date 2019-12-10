package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/database"
)

var (
	spentAddressesDatabase database.Database
)

func configureSpentAddressesDatabase() {
	if db, err := database.Get("spent"); err != nil {
		panic(err)
	} else {
		spentAddressesDatabase = db
	}
}

func databaseKeyForAddress(address trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(address)
}

func spentDatabaseContainsAddress(address trinary.Hash) (bool, error) {
	if contains, err := spentAddressesDatabase.Contains(databaseKeyForAddress(address)); err != nil {
		return contains, errors.Wrap(NewDatabaseError(err), "failed to check if the address exists")
	} else {
		return contains, nil
	}
}

func storeSpentAddressesInDatabase(spent []trinary.Hash) error {

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
		return errors.Wrap(NewDatabaseError(err), "failed to spent addresses")
	}

	return nil
}

// ToDo: stream that directly into the file
func ReadSpentAddressesFromDatabase() ([][]byte, error) {

	var addresses [][]byte
	err := spentAddressesDatabase.ForEach(func(entry database.Entry) (stop bool) {
		address := entry.Key
		addresses = append(addresses, address)
		return false
	})

	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to read spent addresses from database")
	} else {
		return addresses, nil
	}
}
