package snapshot

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {

	// "Which snapshot to load 'local' or 'global'."
	parameter.NodeConfig.SetDefault("loadSnapshot", "local")

	// "Enable local snapshots"
	parameter.NodeConfig.SetDefault("localSnapshots.enabled", true)

	// "The depth, respectively the starting point, at which a local snapshot of the ledger is generated."
	parameter.NodeConfig.SetDefault("localSnapshots.depth", 50)

	// "Interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized"
	parameter.NodeConfig.SetDefault("localSnapshots.intervalSynced", 50)

	// "Interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized"
	parameter.NodeConfig.SetDefault("localSnapshots.intervalUnsynced", 1000)

	// "Path to the local snapshot file"
	parameter.NodeConfig.SetDefault("localSnapshots.path", "latest-export.gz.bin")

	// "Path to the global snapshot file containing the ledger state"
	parameter.NodeConfig.SetDefault("globalSnapshot.path", "snapshotMainnet.txt")

	// "Paths to the spent addresses files"
	parameter.NodeConfig.SetDefault("globalSnapshot.spentAddressesPaths", []string{"previousEpochsSpentAddresses1.txt", "previousEpochsSpentAddresses2.txt", "previousEpochsSpentAddresses3.txt"})

	// "Milestone index of the global snapshot"
	parameter.NodeConfig.SetDefault("globalSnapshot.index", 1050000)

	// "Whether to delete old transaction data from the database."
	parameter.NodeConfig.SetDefault("pruning.enabled", false)

	// "Amount of milestone transactions to keep in the database"
	parameter.NodeConfig.SetDefault("pruning.delay", 40000)

	// "Enable support for wereAddressesSpentFrom (needed for Trinity, but local snapshots are much bigger)"
	parameter.NodeConfig.SetDefault("spentAddresses.enabled", true)
}
