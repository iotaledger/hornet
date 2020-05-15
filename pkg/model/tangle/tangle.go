package tangle

import (
	"errors"

	"go.etcd.io/bbolt"

	"github.com/iotaledger/hive.go/kvstore/bolt"
)

var (
	tangleDb   *bbolt.DB
	snapshotDb *bbolt.DB
	spentDb    *bbolt.DB

	ErrNothingToCleanUp = errors.New("Nothing to clean up in the databases")
)

func boltDB(directory string, filename string) *bbolt.DB {
	db, err := bolt.CreateDB(directory, filename)
	if err != nil {
		panic(err)
	}
	return db
}

func ConfigureDatabases(directory string) {

	tangleDb = boltDB(directory, "tangle.db")

	tangleStore := bolt.New(tangleDb)
	configureHealthStore(tangleStore)
	configureTransactionStorage(tangleStore)
	configureBundleTransactionsStorage(tangleStore)
	configureBundleStorage(tangleStore)
	configureApproversStorage(tangleStore)
	configureTagsStorage(tangleStore)
	configureAddressesStorage(tangleStore)
	configureMilestoneStorage(tangleStore)
	configureUnconfirmedTxStorage(tangleStore)
	configureLedgerStore(tangleStore)

	snapshotDb = boltDB(directory, "snapshot.db")
	snapshotStore := bolt.New(snapshotDb)
	configureSnapshotStore(snapshotStore)

	spentDb = boltDB(directory, "spent.db")
	spentStore := bolt.New(spentDb)
	configureSpentAddressesStorage(spentStore)
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}

func CloseDatabases() error {
	if err := tangleDb.Close(); err != nil {
		return err
	}

	if err := snapshotDb.Close(); err != nil {
		return err
	}

	if err := spentDb.Close(); err != nil {
		return err
	}
	return nil
}

func DatabaseSupportCleanup() bool {
	// Bolt does not support cleaning up anything
	return false
}

func CleanupDatabases() error {
	// Bolt does not support cleaning up anything
	return ErrNothingToCleanUp
}

// GetDatabaseSizes returns the size of the database keys and values.
func GetDatabaseSizes() (tangle int64, snapshot int64, spent int64) {
	//TODO: check filesystem for size
	return 0, 0, 0
}
