package tangle

import (
	"errors"
	"os"
	"path"

	"go.etcd.io/bbolt"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/bolt"

	"github.com/gohornet/hornet/pkg/profile"
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

	snapshotDb = boltDB(directory, SnapshotDbFilename)
	snapshotStore := bolt.New(snapshotDb)

	spentDb = boltDB(directory, SpentAddressesDbFilename)
	spentStore := bolt.New(spentDb)

	ConfigureStorages(tangleStore, snapshotStore, spentStore, profile.LoadProfile().Caches)
}

func ConfigureStorages(tangleStore kvstore.KVStore, snapshotStore kvstore.KVStore, spentStore kvstore.KVStore, caches profile.Caches) {

	configureHealthStore(tangleStore)
	configureTransactionStorage(tangleStore, caches.Transactions)
	configureBundleTransactionsStorage(tangleStore, caches.BundleTransactions)
	configureBundleStorage(tangleStore, caches.Bundles)
	configureApproversStorage(tangleStore, caches.Approvers)
	configureTagsStorage(tangleStore, caches.Tags)
	configureAddressesStorage(tangleStore, caches.Addresses)
	configureMilestoneStorage(tangleStore, caches.Milestones)
	configureUnconfirmedTxStorage(tangleStore, caches.UnconfirmedTx)
	configureLedgerStore(tangleStore)

	configureSnapshotStore(snapshotStore)

	configureSpentAddressesStorage(spentStore, caches.SpentAddresses)
}

func FlushStorages() {
	FlushMilestoneStorage()
	FlushBundleStorage()
	FlushBundleTransactionsStorage()
	FlushTransactionStorage()
	FlushApproversStorage()
	FlushTagsStorage()
	FlushAddressStorage()
	FlushUnconfirmedTxsStorage()
	FlushSpentAddressesStorage()
}

func ShutdownStorages() {

	ShutdownMilestoneStorage()
	ShutdownBundleStorage()
	ShutdownBundleTransactionsStorage()
	ShutdownTransactionStorage()
	ShutdownApproversStorage()
	ShutdownTagsStorage()
	ShutdownAddressStorage()
	ShutdownUnconfirmedTxsStorage()
	ShutdownSpentAddressesStorage()
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
