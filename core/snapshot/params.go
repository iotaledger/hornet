package snapshot

import (
	"time"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hornet/pkg/snapshot"
)

// ParametersSnapshots contains the definition of the parameters used by snapshots.
type ParametersSnapshots struct {
	// Depth defines the depth, respectively the starting point, at which a snapshot of the ledger is generated
	Depth int `default:"50" usage:"the depth, respectively the starting point, at which a snapshot of the ledger is generated"`
	// Interval defines the interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)
	Interval int `default:"200" usage:"interval, in milestones, at which snapshot files are created (snapshots are only created if the node is synced)"`
	// FullPath defines the path to the full snapshot file
	FullPath string `default:"snapshots/mainnet/full_snapshot.bin" usage:"path to the full snapshot file"`
	// DeltaPath defines the path to the delta snapshot file
	DeltaPath string `default:"snapshots/mainnet/delta_snapshot.bin" usage:"path to the delta snapshot file"`
	// DeltaSizeThresholdPercentage defines whether to create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot
	// (0.0 = always create delta snapshot to keep ms diff history)
	DeltaSizeThresholdPercentage float64 `default:"50.0" usage:"create a full snapshot if the size of a delta snapshot reaches a certain percentage of the full snapshot (0.0 = always create delta snapshot to keep ms diff history)"`
	// DownloadURLs defines the URLs to load the snapshot files from.
	DownloadURLs []*snapshot.DownloadTarget `noflag:"true" usage:"URLs to load the snapshot files from"`
}

// ParametersPruning contains the definition of the parameters used by pruning.
type ParametersPruning struct {
	Milestones struct {
		// Enabled defines whether to delete old block data from the database based on maximum milestones to keep
		Enabled bool `default:"false" usage:"whether to delete old block data from the database based on maximum milestones to keep"`
		// MaxMilestonesToKeep defines the maximum amount of milestone cones to keep in the database
		MaxMilestonesToKeep int `default:"60480" usage:"maximum amount of milestone cones to keep in the database"`
	}
	Size struct {
		// Enabled defines whether to delete old block data from the database based on maximum database size
		Enabled bool `default:"true" usage:"whether to delete old block data from the database based on maximum database size"`
		// TargetSize defines the target size of the database
		TargetSize string `default:"30GB" usage:"target size of the database"`
		// ThresholdPercentage defines the percentage the database size gets reduced if the target size is reached
		ThresholdPercentage float64 `default:"10.0" usage:"the percentage the database size gets reduced if the target size is reached"`
		// CooldownTime defines the cooldown time between two pruning by database size events
		CooldownTime time.Duration `default:"5m" usage:"cooldown time between two pruning by database size events"`
	}

	// PruneReceipts defines whether to delete old receipts data from the database
	PruneReceipts bool `default:"false" usage:"whether to delete old receipts data from the database"`
}

var ParamsSnapshots = &ParametersSnapshots{
	DownloadURLs: []*snapshot.DownloadTarget{
		{
			Full:  "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-full_snapshot.bin",
			Delta: "https://chrysalis-dbfiles.iota.org/snapshots/hornet/latest-delta_snapshot.bin",
		},
		{
			Full:  "https://cdn.tanglebay.com/snapshots/mainnet/full_snapshot.bin",
			Delta: "https://cdn.tanglebay.com/snapshots/mainnet/delta_snapshot.bin",
		},
	},
}

var ParamsPruning = &ParametersPruning{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"snapshots": ParamsSnapshots,
		"pruning":   ParamsPruning,
	},
	Masked: nil,
}
