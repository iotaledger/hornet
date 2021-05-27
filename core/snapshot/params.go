package snapshot

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the depth, respectively the starting point, at which a snapshot of the ledger is generated
	CfgSnapshotsDepth = "snapshots.depth"
	// interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)
	CfgSnapshotsInterval = "snapshots.interval"
	// path to the full snapshot file
	CfgSnapshotsFullPath = "snapshots.fullPath"
	// path to the delta snapshot file
	CfgSnapshotsDeltaPath = "snapshots.deltaPath"
	// create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot
	// (0.0 = always create delta snapshot to keep ms diff history)
	CfgSnapshotsDeltaSizeThresholdPercentage = "snapshots.deltaSizeThresholdPercentage"
	// URLs to load the snapshot files from.
	CfgSnapshotsDownloadURLs = "snapshots.downloadURLs"
	// whether to delete old message data from the database
	CfgPruningEnabled = "pruning.enabled"
	// amount of milestone cones to keep in the database (0 = disable milestone based pruning)
	CfgPruningDelay = "pruning.delay"
	// target size of the database (0 = disable database size based pruning)
	CfgPruningTargetDatabaseSize = "pruning.targetDatabaseSize"
	// the percentage the database size gets reduced if the target size is reached
	CfgPruningTargetDatabaseSizeThresholdPercentage = "pruning.targetDatabaseSizeThresholdPercentage"
	// whether to delete old receipts data from the database
	CfgPruningPruneReceipts = "pruning.pruneReceipts"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgSnapshotsDepth, 50, "the depth, respectively the starting point, at which a snapshot of the ledger is generated")
			fs.Int(CfgSnapshotsInterval, 50, "interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)")
			fs.String(CfgSnapshotsFullPath, "snapshots/mainnet/full_snapshot.bin", "path to the full snapshot file")
			fs.String(CfgSnapshotsDeltaPath, "snapshots/mainnet/delta_snapshot.bin", "path to the delta snapshot file")
			fs.Float64(CfgSnapshotsDeltaSizeThresholdPercentage, 50.0, "create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot (0.0 = always create delta snapshot to keep ms diff history)")
			fs.Bool(CfgPruningEnabled, true, "whether to delete old message data from the database")
			fs.Int(CfgPruningDelay, 60480, "amount of milestone cones to keep in the database (0 = disable milestone based pruning)")
			fs.String(CfgPruningTargetDatabaseSize, "35GB", "target size of the database (0 = disable database size based pruning)")
			fs.Float64(CfgPruningTargetDatabaseSizeThresholdPercentage, 5.0, "the percentage the database size gets reduced if the target size is reached")
			fs.Bool(CfgPruningPruneReceipts, false, "whether to delete old receipts data from the database")
			return fs
		}(),
	},
	Masked: nil,
}
