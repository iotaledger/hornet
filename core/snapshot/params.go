package snapshot

import (
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

var params = &app.ComponentParams{
	Params: map[string]any{
		"snapshots": ParamsSnapshots,
	},
	Masked: nil,
}
