package tangle

import (
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/profile"
)

func ConfigureDatabases(directory string, badgerOpts *profile.BadgerOpts) {
	database.Settings(directory, badgerOpts)
	configureHealthDatabase()
	configureTransactionStorage()
	configureBundleDatabase()
	configureTransactionHashesForAddressDatabase()
	configureApproversStorage()
	configureMilestoneDatabase()
	configureLedgerDatabase()
	configureSnapshotDatabase()
	configureTransactionHashesForAddressDatabase()
	configureFirstSeenTransactionsDatabase()
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}
