package config

const (
	// which snapshot type to load. 'local' or 'global'
	CfgSnapshotLoadType = "snapshots.loadType"
	// the depth, respectively the starting point, at which a local snapshot of the ledger is generated
	CfgLocalSnapshotsDepth = "snapshots.local.depth"
	// interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized
	CfgLocalSnapshotsIntervalSynced = "snapshots.local.intervalSynced"
	// interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized
	CfgLocalSnapshotsIntervalUnsynced = "snapshots.local.intervalUnsynced"
	// path to the local snapshot file
	CfgLocalSnapshotsPath = "snapshots.local.path"
	// URL to load the local snapshot file from
	CfgLocalSnapshotsDownloadURLs = "snapshots.local.downloadURLs"
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
	configFlagSet.String(CfgSnapshotLoadType, "local", "which snapshot type to load. 'local' or 'global'")
	configFlagSet.Int(CfgLocalSnapshotsDepth, 50, "the depth, respectively the starting point, at which a local snapshot of the ledger is generated")
	configFlagSet.Int(CfgLocalSnapshotsIntervalSynced, 50, "interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized")
	configFlagSet.Int(CfgLocalSnapshotsIntervalUnsynced, 1000, "interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized")
	configFlagSet.String(CfgLocalSnapshotsPath, "snapshots/mainnet/export.bin", "path to the local snapshot file")
	configFlagSet.StringSlice(CfgLocalSnapshotsDownloadURLs, []string{}, "URLs to load the local snapshot file from. Provide multiple URLs as fall back sources")
	configFlagSet.String(CfgGlobalSnapshotPath, "snapshotMainnet.txt", "path to the global snapshot file containing the ledger state")
	configFlagSet.StringSlice(CfgGlobalSnapshotSpentAddressesPaths, []string{
		"previousEpochsSpentAddresses1.txt",
		"previousEpochsSpentAddresses2.txt",
		"previousEpochsSpentAddresses3.txt",
	}, "paths to the spent addresses files")
	configFlagSet.Int(CfgGlobalSnapshotIndex, 1050000, "milestone index of the global snapshot")
	configFlagSet.Bool(CfgPruningEnabled, true, "whether to delete old transaction data from the database")
	configFlagSet.Int(CfgPruningDelay, 60480, "amount of milestone transactions to keep in the database")
	configFlagSet.Bool(CfgSpentAddressesEnabled, true, "enable support for wereAddressesSpentFrom (needed for Trinity, but local snapshots are much bigger)")
}
