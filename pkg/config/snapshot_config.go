package config

const (
	// the depth, respectively the starting point, at which a snapshot of the ledger is generated
	CfgSnapshotsDepth = "snapshots.depth"
	// interval, in milestones, at which snapshot files are created if the ledger is fully synchronized
	CfgSnapshotsIntervalSynced = "snapshots.intervalSynced"
	// interval, in milestones, at which snapshot files are created if the ledger is not fully synchronized
	CfgSnapshotsIntervalUnsynced = "snapshots.intervalUnsynced"
	// path to the snapshot file
	CfgSnapshotsPath = "snapshots.path"
	// URL to load the snapshot file from
	CfgSnapshotsDownloadURLs = "snapshots.downloadURLs"
)

func init() {
	configFlagSet.Int(CfgSnapshotsDepth, 50, "the depth, respectively the starting point, at which a snapshot of the ledger is generated")
	configFlagSet.Int(CfgSnapshotsIntervalSynced, 50, "interval, in milestones, at which snapshot files are created if the ledger is fully synchronized")
	configFlagSet.Int(CfgSnapshotsIntervalUnsynced, 1000, "interval, in milestones, at which snapshot files are created if the ledger is not fully synchronized")
	configFlagSet.String(CfgSnapshotsPath, "snapshots/mainnet/export.bin", "path to the snapshot file")
	configFlagSet.StringSlice(CfgSnapshotsDownloadURLs, []string{}, "URLs to load the snapshot file from. Provide multiple URLs as fall back sources")
}
