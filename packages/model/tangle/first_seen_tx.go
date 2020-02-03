package tangle

import (
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var (
	firstSeenTransactionDatabase database.Database
)

func configureFirstSeenTransactionsDatabase() {
	if db, err := database.Get(DBPrefixFirstSeenTransactions, database.GetHornetBadgerInstance()); err != nil {
		panic(err)
	} else {
		firstSeenTransactionDatabase = db
	}
}

func databaseKeyPrefixForFirstSeenTransaction(milestoneIndex milestone_index.MilestoneIndex) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(milestoneIndex))
	return bytes
}

func databaseKeyForFirstSeenTransaction(milestoneIndex milestone_index.MilestoneIndex, txHash trinary.Hash) []byte {
	return append(databaseKeyPrefixForFirstSeenTransaction(milestoneIndex), trinary.MustTrytesToBytes(txHash)[:49]...)
}

func transactionHashFromDatabaseKey(transactionHash []byte) trinary.Hash {
	return trinary.MustBytesToTrytes(transactionHash, 81)
}

type FirstSeenTxHashOperation struct {
	FirstSeenLatestMilestoneIndex milestone_index.MilestoneIndex
	TxHash                        trinary.Hash
}

func StoreFirstSeenTxHashOperations(operations []*FirstSeenTxHashOperation) error {

	// Create entries for all txs
	var entries []database.Entry

	for _, op := range operations {
		entry := database.Entry{
			Key: databaseKeyForFirstSeenTransaction(op.FirstSeenLatestMilestoneIndex, op.TxHash),
		}
		entries = append(entries, entry)
	}

	// Now batch insert all entries
	if err := firstSeenTransactionDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store first seen tx in database")
	}

	return nil
}

func FixFirstSeenTxHashOperations(firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex) error {

	var entries []database.Entry
	var deletions []database.Key

	// Search all entries with milestone 0
	err := firstSeenTransactionDatabase.StreamForEachPrefixKeyOnly(databaseKeyPrefixForFirstSeenTransaction(0), func(entry database.KeyOnlyEntry) error {
		newEntry := database.Entry{
			Key: append(databaseKeyPrefixForFirstSeenTransaction(firstSeenLatestMilestoneIndex), entry.Key[4:]...),
		}
		entries = append(entries, newEntry)
		deletions = append(deletions, entry.Key)
		return nil
	})

	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to fix first seen tx")
	}

	// Now batch insert all entries
	if err := firstSeenTransactionDatabase.Apply(entries, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to fix first seen tx in database")
	}

	return nil
}

func DeleteFirstSeenTxHashOperations(firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex) error {
	var deletions []database.Key

	err := firstSeenTransactionDatabase.StreamForEachPrefixKeyOnly(databaseKeyPrefixForFirstSeenTransaction(firstSeenLatestMilestoneIndex), func(entry database.KeyOnlyEntry) error {
		deletions = append(deletions, entry.Key)
		return nil
	})

	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete first seen tx")
	}

	// Now batch delete all entries
	if err := firstSeenTransactionDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete first seen tx")
	}

	return nil
}

func ReadFirstSeenTxHashOperations(firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex) ([]trinary.Hash, error) {

	var transactionHashes []trinary.Hash

	err := firstSeenTransactionDatabase.StreamForEachPrefixKeyOnly(databaseKeyPrefixForFirstSeenTransaction(firstSeenLatestMilestoneIndex), func(entry database.KeyOnlyEntry) error {
		transactionHashes = append(transactionHashes, transactionHashFromDatabaseKey(entry.Key[4:]))
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to read first seen tx from database")
	}

	return transactionHashes, nil
}
