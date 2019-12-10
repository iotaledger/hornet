package tangle

import "github.com/gohornet/hornet/packages/database"

const (
	BundleCacheSize         = 20000
	MilestoneCacheSize      = 1000
	TransactionCacheSize    = 50000
	ApproversCacheSize      = 100000
	SpentAddressesCacheSize = 5000
)

func ConfigureDatabases(directory string, light bool) {
	database.Settings(directory, light)
	configureHealthDatabase()
	configureTransactionDatabase()
	configureBundleDatabase()
	configureTransactionHashesForAddressDatabase()
	configureApproversDatabase()
	configureMilestoneDatabase()
	configureLedgerDatabase()
	configureSnapshotDatabase()
	configureSpentAddressesDatabase()
	configureTransactionHashesForAddressDatabase()
	configureUnconfirmedTransactionsDatabase()
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}
