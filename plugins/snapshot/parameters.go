package snapshot

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
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

	// "Whether to load a global snapshot from provided text files."
	parameter.NodeConfig.SetDefault("globalSnapshot.load", false)

	// "Path to the global snapshot file"
	parameter.NodeConfig.SetDefault("globalSnapshot.path", "snapshotMainnet.txt")

	// "Paths to the spent addresses files"
	parameter.NodeConfig.SetDefault("globalSnapshot.spentAddressesPaths", []string{"previousEpochsSpentAddresses1.txt", "previousEpochsSpentAddresses2.txt", "previousEpochsSpentAddresses3.txt"})

	// "Milestone index of the global snapshot"
	parameter.NodeConfig.SetDefault("globalSnapshot.index", 1050000)

	// "Whether to delete old transaction data from the database."
	parameter.NodeConfig.SetDefault("pruning.enabled", false)

	// "Amount of milestone transactions to keep in the database"
	parameter.NodeConfig.SetDefault("pruning.delay", 40000)

	// "Path to the ledger state file for your private tangle"
	parameter.NodeConfig.SetDefault("privateTangle.ledgerStatePath", "balances.txt")
}
