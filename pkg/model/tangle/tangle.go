package tangle

import (
	"github.com/gohornet/hornet/pkg/store"
)

func ConfigureDatabases(directory string) {
	store.Settings(directory)
	configureHealthStore()
	configureTransactionStorage()
	configureBundleTransactionsStorage()
	configureBundleStorage()
	configureApproversStorage()
	configureTagsStorage()
	configureAddressesStorage()
	configureMilestoneStorage()
	configureUnconfirmedTxStorage()
	configureLedgerStore()
	configureSnapshotStore()
	configureSpentAddressesStorage()
}

func LoadInitialValuesFromDatabase() {
	loadSnapshotInfo()
	loadSolidEntryPoints()
}
