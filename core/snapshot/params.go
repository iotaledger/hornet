package snapshot

import (
	"time"

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
	// whether to delete old message data from the database based on maximum milestones to keep
	CfgPruningMilestonesEnabled = "pruning.milestones.enabled"
	// maximum amount of milestone cones to keep in the database
	CfgPruningMilestonesMaxMilestonesToKeep = "pruning.milestones.maxMilestonesToKeep"
	// whether to delete old message data from the database based on maximum database size
	CfgPruningSizeEnabled = "pruning.size.enabled"
	// target size of the database
	CfgPruningSizeTargetSize = "pruning.size.targetSize"
	// the percentage the database size gets reduced if the target size is reached
	CfgPruningSizeThresholdPercentage = "pruning.size.thresholdPercentage"
	// cooldown time between two pruning by database size events
	CfgPruningSizeCooldownTime = "pruning.size.cooldownTime"
	// whether to delete old receipts data from the database
	CfgPruningPruneReceipts = "pruning.pruneReceipts"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgSnapshotsDepth, 50, "the depth, respectively the starting point, at which a snapshot of the ledger is generated")
			fs.Int(CfgSnapshotsInterval, 200, "interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)")
			fs.String(CfgSnapshotsFullPath, "snapshots/mainnet/full_snapshot.bin", "path to the full snapshot file")
			fs.String(CfgSnapshotsDeltaPath, "snapshots/mainnet/delta_snapshot.bin", "path to the delta snapshot file")
			fs.Float64(CfgSnapshotsDeltaSizeThresholdPercentage, 50.0, "create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot (0.0 = always create delta snapshot to keep ms diff history)")
			fs.Bool(CfgPruningMilestonesEnabled, false, "whether to delete old message data from the database based on maximum milestones to keep")
			fs.Int(CfgPruningMilestonesMaxMilestonesToKeep, 60480, "maximum amount of milestone cones to keep in the database")
			fs.Bool(CfgPruningSizeEnabled, true, "whether to delete old message data from the database based on maximum database size")
			fs.String(CfgPruningSizeTargetSize, "30GB", "target size of the database")
			fs.Float64(CfgPruningSizeThresholdPercentage, 10.0, "the percentage the database size gets reduced if the target size is reached")
			fs.Duration(CfgPruningSizeCooldownTime, 5*time.Minute, "cooldown time between two pruning by database size events")
			fs.Bool(CfgPruningPruneReceipts, false, "whether to delete old receipts data from the database")
			return fs
		}(),
	},
	Masked: nil,
}
