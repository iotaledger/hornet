package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/database"
)

var (
	transactionsHashesForAddressDatabase database.Database
)

func configureTransactionHashesForAddressDatabase() {
	if db, err := database.Get("address"); err != nil {
		panic(err)
	} else {
		transactionsHashesForAddressDatabase = db
	}
}

type TxHashForAddress struct {
	Address trinary.Hash
	TxHash  trinary.Hash
}

func StoreTransactionHashesForAddressesInDatabase(addresses []*TxHashForAddress) error {

	// Create entries for all txs in all addresses
	var entries []database.Entry
	for _, address := range addresses {
		entry := database.Entry{
			Key:   databaseKeyForHashPrefixedHash(address.Address, address.TxHash),
			Value: []byte{},
			Meta:  0,
		}
		entries = append(entries, entry)
	}

	// Now batch insert all entries
	if err := transactionsHashesForAddressDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store txs for addresses in database")
	}

	return nil
}

func DeleteTransactionHashesForAddressesInDatabase(addresses []*TxHashForAddress) error {
	var deletions []database.Key

	for _, address := range addresses {
		deletions = append(deletions, databaseKeyForHashPrefixedHash(address.Address, address.TxHash))
	}

	// Now batch delete all entries
	if err := transactionsHashesForAddressDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete txs for addresses")
	}

	return nil
}

func ReadTransactionHashesForAddressFromDatabase(address trinary.Hash, maxNumber int) ([]trinary.Hash, error) {

	var transactionHashes []trinary.Hash
	err := transactionsHashesForAddressDatabase.ForEachPrefixKeyOnly(databaseKeyForHashPrefix(address), func(entry database.KeyOnlyEntry) (stop bool) {
		txHash := trinary.MustBytesToTrytes(entry.Key, 81)
		transactionHashes = append(transactionHashes, txHash)
		return len(transactionHashes) >= maxNumber
	})

	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to read tx per address from database")
	} else {
		return transactionHashes, nil
	}
}
