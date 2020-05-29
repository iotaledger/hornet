package tangle

import (
	"errors"
	"os"
	"path"

	"go.etcd.io/bbolt"

	"github.com/iotaledger/hive.go/kvstore/bolt"
)

const (
	TangleDbFilename         = "tangle.db"
	SnapshotDbFilename       = "snapshot.db"
	SpentAddressesDbFilename = "spent.db"
)

var (
	dbDir      string
	tangleDb   *bbolt.DB
	snapshotDb *bbolt.DB
	spentDb    *bbolt.DB

	ErrNothingToCleanUp = errors.New("Nothing to clean up in the databases")
)

func boltDB(directory string, filename string) *bbolt.DB {
	opts := &bbolt.Options{
		NoSync: true,
	}
	db, err := bolt.CreateDB(directory, filename, opts)
	if err != nil {
		panic(err)
	}
	return db
}

func ConfigureDatabases(directory string) {

	dbDir = directory
	tangleDb = boltDB(directory, TangleDbFilename)

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

	snapshotDb = boltDB(directory, SnapshotDbFilename)
	snapshotStore := bolt.New(snapshotDb)
	configureSnapshotStore(snapshotStore)

	spentDb = boltDB(directory, SpentAddressesDbFilename)
	spentStore := bolt.New(spentDb)
	configureSpentAddressesStorage(spentStore)
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}

func CloseDatabases() error {

	if err := tangleDb.Sync(); err != nil {
		return err
	}

	if err := tangleDb.Close(); err != nil {
		return err
	}

	if err := snapshotDb.Sync(); err != nil {
		return err
	}

	if err := snapshotDb.Close(); err != nil {
		return err
	}

	if err := spentDb.Sync(); err != nil {
		return err
	}

	if err := spentDb.Close(); err != nil {
		return err
	}
	return nil
}

func DatabaseSupportsCleanup() bool {
	// Bolt does not support cleaning up anything
	return false
}

func CleanupDatabases() error {
	// Bolt does not support cleaning up anything
	return ErrNothingToCleanUp
}

// GetDatabaseSizes returns the size of the different databases.
func GetDatabaseSizes() (tangle int64, snapshot int64, spent int64) {

	if tangleDbFile, err := os.Stat(path.Join(dbDir, TangleDbFilename)); err == nil {
		tangle = tangleDbFile.Size()
	}

	if snapshotDbFile, err := os.Stat(path.Join(dbDir, SnapshotDbFilename)); err == nil {
		snapshot = snapshotDbFile.Size()
	}

	if spentDbFile, err := os.Stat(path.Join(dbDir, SpentAddressesDbFilename)); err == nil {
		spent = spentDbFile.Size()
	}

	return
}
