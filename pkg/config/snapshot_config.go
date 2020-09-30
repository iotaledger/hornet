package config

const (
	// the depth, respectively the starting point, at which a local snapshot of the ledger is generated
	CfgLocalSnapshotsDepth = "snapshots.depth"
	// interval, in milestones, at which snapshot files are created if the ledger is fully synchronized
	CfgLocalSnapshotsIntervalSynced = "snapshots.intervalSynced"
	// interval, in milestones, at which snapshot files are created if the ledger is not fully synchronized
	CfgLocalSnapshotsIntervalUnsynced = "snapshots.intervalUnsynced"
	// path to the local snapshot file
	CfgLocalSnapshotsPath = "snapshots.path"
	// URL to load the local snapshot file from
	CfgLocalSnapshotsDownloadURLs = "snapshots.downloadURLs"
)

func init() {
	configFlagSet.Int(CfgLocalSnapshotsDepth, 50, "the depth, respectively the starting point, at which a local snapshot of the ledger is generated")
	configFlagSet.Int(CfgLocalSnapshotsIntervalSynced, 50, "interval, in milestones, at which snapshot files are created if the ledger is fully synchronized")
	configFlagSet.Int(CfgLocalSnapshotsIntervalUnsynced, 1000, "interval, in milestones, at which snapshot files are created if the ledger is not fully synchronized")
	configFlagSet.String(CfgLocalSnapshotsPath, "snapshots/mainnet/export.bin", "path to the local snapshot file")
	configFlagSet.StringSlice(CfgLocalSnapshotsDownloadURLs, []string{}, "URLs to load the local snapshot file from. Provide multiple URLs as fall back sources")
}
