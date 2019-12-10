package tangle

import (
	"encoding/binary"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var transactionDatabase database.Database

func configureTransactionDatabase() {
	if db, err := database.Get("tx"); err != nil {
		panic(err)
	} else {
		transactionDatabase = db
	}
}

func databaseKeyForTransaction(transaction *hornet.Transaction) []byte {
	return trinary.MustTrytesToBytes(transaction.GetHash())
}

func databaseKeyForTransactionHash(transactionHash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(transactionHash)
}

func transactionHashFromDatabaseKey(transactionHash []byte) trinary.Hash {
	return trinary.MustBytesToTrytes(transactionHash, 81)
}

func StoreTransactionsInDatabase(transactions []*hornet.Transaction) error {

	// Create entries for all tx
	var entries []database.Entry
	var modifiedTx []*hornet.Transaction
	for _, transaction := range transactions {
		if transaction.IsModified() {
			value := make([]byte, 4, 4+len(transaction.RawBytes))
			confirmed, confirmationIndex := transaction.GetConfirmed()

			if !confirmed {
				confirmationIndex = 0
			}

			binary.LittleEndian.PutUint32(value, uint32(confirmationIndex))
			value = append(value, transaction.RawBytes...)

			entry := database.Entry{
				Key:   databaseKeyForTransaction(transaction),
				Value: value,
				Meta:  transaction.GetMetadata(),
			}
			entries = append(entries, entry)
			modifiedTx = append(modifiedTx, transaction)
		}
	}

	// Now batch insert all entries
	if err := transactionDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store transactions")
	}

	// Mark all tx as not modified after persisting them
	for _, tx := range modifiedTx {
		tx.SetModified(false)
	}

	return nil
}

func DeleteTransactionsInDatabase(transactionHashes []trinary.Hash) error {
	var deletions []database.Key

	for _, transactionHash := range transactionHashes {
		deletions = append(deletions, databaseKeyForTransactionHash(transactionHash))
	}

	// Now batch delete all entries
	if err := transactionDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete transactions")
	}

	return nil
}

func readTransactionFromDatabase(transactionHash trinary.Hash) (*hornet.Transaction, error) {
	entry, err := transactionDatabase.Get(databaseKeyForTransactionHash(transactionHash))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve transaction")
		}
	}

	confirmationIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(entry.Value[:4]))
	rawBytes := entry.Value[4:]

	tx, err := compressed.TransactionFromCompressedBytes(rawBytes, transactionHash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decompress tx")
	} else {
		return hornet.NewTransactionFromDatabase(tx, rawBytes, confirmationIndex, entry.Meta), nil
	}
}

func databaseContainsTransaction(transactionHash trinary.Hash) (bool, error) {
	if contains, err := transactionDatabase.Contains(databaseKeyForTransactionHash(transactionHash)); err != nil {
		return contains, errors.Wrap(NewDatabaseError(err), "failed to check if the transaction exists")
	} else {
		return contains, nil
	}
}
