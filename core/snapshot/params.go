package snapshot

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// whether to delete old message data from the database
	CfgPruningEnabled = "pruning.enabled"
	// amount of milestone cones to keep in the database
	CfgPruningDelay = "pruning.delay"

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

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Bool(CfgPruningEnabled, true, "whether to delete old message data from the database")
			fs.Int(CfgPruningDelay, 60480, "amount of milestone cones to keep in the database")
			fs.Int(CfgSnapshotsDepth, 50, "the depth, respectively the starting point, at which a snapshot of the ledger is generated")
			fs.Int(CfgSnapshotsIntervalSynced, 50, "interval, in milestones, at which snapshot files are created if the ledger is fully synchronized")
			fs.Int(CfgSnapshotsIntervalUnsynced, 1000, "interval, in milestones, at which snapshot files are created if the ledger is not fully synchronized")
			fs.String(CfgSnapshotsPath, "snapshots/mainnet/export.bin", "path to the snapshot file")
			fs.StringSlice(CfgSnapshotsDownloadURLs, []string{}, "URLs to load the snapshot file from. Provide multiple URLs as fall back sources")
			return fs
		}(),
	},
	Masked: nil,
}
