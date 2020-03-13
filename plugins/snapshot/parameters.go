package snapshot

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {

	// which snapshot type to load. 'local' or 'global'
	config.NodeConfig.SetDefault(config.CfgSnapshotLoadType, "local")

	// whether to do local snapshots
	config.NodeConfig.SetDefault(config.CfgLocalSnapshotsEnabled, true)

	// the depth, respectively the starting point, at which a local snapshot of the ledger is generated
	config.NodeConfig.SetDefault(config.CfgLocalSnapshotsDepth, 50)

	// interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized
	config.NodeConfig.SetDefault(config.CfgLocalSnapshotsIntervalSynced, 50)

	// interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized
	config.NodeConfig.SetDefault(config.CfgLocalSnapshotsIntervalUnsynced, 1000)

	// path to the local snapshot file
	config.NodeConfig.SetDefault(config.CfgLocalSnapshotsPath, "latest-export.gz.bin")

	// path to the global snapshot file containing the ledger state
	config.NodeConfig.SetDefault(config.CfgGlobalSnapshotPath, "snapshotMainnet.txt")

	// paths to the spent addresses files
	config.NodeConfig.SetDefault(config.CfgGlobalSnapshotSpentAddressesPath, []string{
		"previousEpochsSpentAddresses1.txt",
		"previousEpochsSpentAddresses2.txt",
		"previousEpochsSpentAddresses3.txt",
	})

	// milestone index of the global snapshot
	config.NodeConfig.SetDefault(config.CfgGlobalSnapshotIndex, 1050000)

	// whether to delete old transaction data from the database
	config.NodeConfig.SetDefault(config.CfgPruningEnabled, false)

	// amount of milestone transactions to keep in the database
	config.NodeConfig.SetDefault(config.CfgPruningDelay, 40000)

	// enable support for wereAddressesSpentFrom (needed for Trinity, but local snapshots are much bigger)
	config.NodeConfig.SetDefault(config.CfgSpentAddressesEnabled, true)
}
