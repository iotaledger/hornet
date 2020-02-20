package tangle

import (
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/profile"
)

func ConfigureDatabases(directory string, badgerOpts *profile.BadgerOpts) {
	database.Settings(directory, badgerOpts)
	configureHealthDatabase()
	configureTransactionRawStorage()
	configureTransactionMetaStorage()
	configureBundleTransactionsStorage()
	configureBundleStorage()
	configureApproversStorage()
	configureTagsStorage()
	configureAddressesStorage()
	configureMilestoneStorage()
	configureLedgerDatabase()
	configureSnapshotDatabase()
	configureFirstSeenTxStorage()
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}
