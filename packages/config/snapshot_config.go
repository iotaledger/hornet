package config

const (
	// which snapshot type to load. 'local' or 'global'
	CfgSnapshotLoadType = "snapshots.loadType"
	// whether to do local snapshots
	CfgLocalSnapshotsEnabled = "snapshots.local.enabled"
	// the depth, respectively the starting point, at which a local snapshot of the ledger is generated
	CfgLocalSnapshotsDepth = "snapshots.local.depth"
	// interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized
	CfgLocalSnapshotsIntervalSynced = "snapshots.local.intervalSynced"
	// interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized
	CfgLocalSnapshotsIntervalUnsynced = "snapshots.local.intervalUnsynced"
	// path to the local snapshot file
	CfgLocalSnapshotsPath = "snapshots.local.path"
	// path to the global snapshot file containing the ledger state
	CfgGlobalSnapshotPath = "snapshots.global.path"
	// paths to the spent addresses files
	CfgGlobalSnapshotSpentAddressesPaths = "snapshots.global.spentAddressesPaths"
	// milestone index of the global snapshot
	CfgGlobalSnapshotIndex = "snapshots.global.index"
	// whether to delete old transaction data from the database
	CfgPruningEnabled = "snapshots.pruning.enabled"
	// amount of milestone transactions to keep in the database
	CfgPruningDelay = "snapshots.pruning.delay"
	// enable support for wereAddressesSpentFrom (needed for Trinity, but local snapshots are much bigger)
	CfgSpentAddressesEnabled = "spentAddresses.enabled"
)

func init() {
	NodeConfig.SetDefault(CfgSnapshotLoadType, "local")
	NodeConfig.SetDefault(CfgLocalSnapshotsEnabled, true)
	NodeConfig.SetDefault(CfgLocalSnapshotsDepth, 50)
	NodeConfig.SetDefault(CfgLocalSnapshotsIntervalSynced, 50)
	NodeConfig.SetDefault(CfgLocalSnapshotsIntervalUnsynced, 1000)
	NodeConfig.SetDefault(CfgLocalSnapshotsPath, "export.bin")
	NodeConfig.SetDefault(CfgGlobalSnapshotPath, "snapshotMainnet.txt")
	NodeConfig.SetDefault(CfgGlobalSnapshotSpentAddressesPaths, []string{
		"previousEpochsSpentAddresses1.txt",
		"previousEpochsSpentAddresses2.txt",
		"previousEpochsSpentAddresses3.txt",
	})
	NodeConfig.SetDefault(CfgGlobalSnapshotIndex, 1050000)
	NodeConfig.SetDefault(CfgPruningEnabled, false)
	NodeConfig.SetDefault(CfgPruningDelay, 40000)
	NodeConfig.SetDefault(CfgSpentAddressesEnabled, true)
}
