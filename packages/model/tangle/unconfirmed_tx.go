package tangle

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/database"

	hornetDB "github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var (
	unconfirmedTransactionDatabase database.Database
)

func configureUnconfirmedTransactionsDatabase() {
	if db, err := database.Get(DBPrefixUnconfirmedTransactions, hornetDB.GetBadgerInstance()); err != nil {
		panic(err)
	} else {
		unconfirmedTransactionDatabase = db
	}
}

type UnconfirmedTxHashOperation struct {
	TxHash                        trinary.Hash
	FirstSeenLatestMilestoneIndex milestone_index.MilestoneIndex
	Confirmed                     bool
}

func StoreUnconfirmedTxHashOperations(operations []*UnconfirmedTxHashOperation) error {

	// Create entries for all txs in all addresses
	var entries []database.Entry
	var deletions []database.Key
	for _, op := range operations {

		key := databaseKeyForTransactionHash(op.TxHash)
		if op.Confirmed {
			deletions = append(deletions, key)
		} else {
			entry := database.Entry{
				Key:   key,
				Value: bytesFromMilestoneIndex(op.FirstSeenLatestMilestoneIndex),
				Meta:  0,
			}
			entries = append(entries, entry)
		}
	}

	// Now batch insert all entries
	if err := unconfirmedTransactionDatabase.Apply(entries, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store unconfirmed tx in database")
	}

	return nil
}

func DeleteUnconfirmedTxHashOperations(transactionHashes []trinary.Hash) error {
	var deletions []database.Key

	for _, txHash := range transactionHashes {
		deletions = append(deletions, databaseKeyForTransactionHash(txHash))
	}

	// Now batch delete all entries
	if err := unconfirmedTransactionDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete unconfirmed txs")
	}

	return nil
}

func ReadUnconfirmedTxHashOperations(milestoneIndex milestone_index.MilestoneIndex) ([]trinary.Hash, error) {

	var transactionHashes []trinary.Hash

	err := unconfirmedTransactionDatabase.StreamForEach(func(entry database.Entry) error {
		index := milestoneIndexFromBytes(entry.Value)
		if index <= milestoneIndex {
			transactionHashes = append(transactionHashes, transactionHashFromDatabaseKey(entry.Key))
		}
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to read unconfirmed tx from database")
	}

	return transactionHashes, nil
}
