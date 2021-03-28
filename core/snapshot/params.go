package snapshot

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// whether to delete old message data from the database
	CfgPruningEnabled = "pruning.enabled"
	// amount of milestone cones to keep in the database
	CfgPruningDelay = "pruning.delay"
	// whether to delete old receipts data from the database
	CfgPruningPruneReceipts = "pruning.pruneReceipts"
	// the depth, respectively the starting point, at which a snapshot of the ledger is generated
	CfgSnapshotsDepth = "snapshots.depth"
	// interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)
	CfgSnapshotsInterval = "snapshots.interval"
	// path to the full snapshot file
	CfgSnapshotsFullPath = "snapshots.fullPath"
	// path to the delta snapshot file
	CfgSnapshotsDeltaPath = "snapshots.deltaPath"
	// URLs to load the snapshot files from.
	CfgSnapshotsDownloadURLs = "snapshots.downloadURLs"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Bool(CfgPruningEnabled, true, "whether to delete old message data from the database")
			fs.Int(CfgPruningDelay, 60480, "amount of milestone cones to keep in the database")
			fs.Bool(CfgPruningPruneReceipts, false, "whether to delete old receipts data from the database")
			fs.Int(CfgSnapshotsDepth, 50, "the depth, respectively the starting point, at which a snapshot of the ledger is generated")
			fs.Int(CfgSnapshotsInterval, 50, "interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)")
			fs.String(CfgSnapshotsFullPath, "snapshots/mainnet/full_export.bin", "path to the full snapshot file")
			fs.String(CfgSnapshotsDeltaPath, "snapshots/mainnet/delta_export.bin", "path to the delta snapshot file")
			return fs
		}(),
	},
	Masked: nil,
}
