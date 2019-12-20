package snapshot

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Enable local snapshots"
	parameter.NodeConfig.SetDefault("localSnapshots.enabled", true)

	// "Enable your node to delete old transactions from its database"
	parameter.NodeConfig.SetDefault("localSnapshots.pruningEnabled", true)

	// "Amount of seen milestones to record in the snapshot file"
	parameter.NodeConfig.SetDefault("localSnapshots.depth", 100)

	// "Amount of milestone transactions to keep in the ledger"
	parameter.NodeConfig.SetDefault("localSnapshots.pruningDelay", 40000)

	// "Interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized"
	parameter.NodeConfig.SetDefault("localSnapshots.intervalSynced", 10)

	// "Interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized"
	parameter.NodeConfig.SetDefault("localSnapshots.intervalUnsynced", 1000)

	// "Path to the snapshot file"
	parameter.NodeConfig.SetDefault("localSnapshots.path", "latest-export.gz.bin")
}
