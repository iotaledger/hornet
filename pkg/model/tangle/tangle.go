package tangle

import (
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/profile"
)

func ConfigureDatabases(directory string, badgerOpts *profile.BadgerOpts) {
	database.Settings(directory, badgerOpts)
	configureHealthDatabase()
	configureTransactionStorage()
	configureBundleTransactionsStorage()
	configureBundleStorage()
	configureApproversStorage()
	configureTagsStorage()
	configureAddressesStorage()
	configureMilestoneStorage()
	configureUnconfirmedTxStorage()
	configureLedgerDatabase()
	configureSnapshotDatabase()
	configureSpentAddressesStorage()
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}
