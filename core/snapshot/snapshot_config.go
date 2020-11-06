package snapshot

import "github.com/gohornet/hornet/core/cli"

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
	cli.ConfigFlagSet.Int(CfgSnapshotsDepth, 50, "the depth, respectively the starting point, at which a snapshot of the ledger is generated")
	cli.ConfigFlagSet.Int(CfgSnapshotsIntervalSynced, 50, "interval, in milestones, at which snapshot files are created if the ledger is fully synchronized")
	cli.ConfigFlagSet.Int(CfgSnapshotsIntervalUnsynced, 1000, "interval, in milestones, at which snapshot files are created if the ledger is not fully synchronized")
	cli.ConfigFlagSet.String(CfgSnapshotsPath, "snapshots/mainnet/export.bin", "path to the snapshot file")
	cli.ConfigFlagSet.StringSlice(CfgSnapshotsDownloadURLs, []string{}, "URLs to load the snapshot file from. Provide multiple URLs as fall back sources")
}
