package snapshot

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Enable local snapshots"
	parameter.NodeConfig.SetDefault("localSnapshots.enabled", true)

	// "Amount of seen milestones to record in the local snapshot file"
	parameter.NodeConfig.SetDefault("localSnapshots.depth", 100)

	// "Interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized"
	parameter.NodeConfig.SetDefault("localSnapshots.intervalSynced", 10)

	// "Interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized"
	parameter.NodeConfig.SetDefault("localSnapshots.intervalUnsynced", 1000)

	// "Path to the local snapshot file"
	parameter.NodeConfig.SetDefault("localSnapshots.path", "latest-export.gz.bin")

	// "Load global snapshot"
	parameter.NodeConfig.SetDefault("globalSnapshot.load", false)

	// "Path to the global snapshot file"
	parameter.NodeConfig.SetDefault("globalSnapshot.path", "snapshotMainnet.txt")

	// "Paths to the spent addresses files"
	parameter.NodeConfig.SetDefault("globalSnapshot.spentAddressesPaths", []string{"previousEpochsSpentAddresses1.txt", "previousEpochsSpentAddresses2.txt", "previousEpochsSpentAddresses3.txt"})

	// "Milestone index of the global snapshot"
	parameter.NodeConfig.SetDefault("globalSnapshot.index", 1050000)

	// "Delete old transactions from database"
	parameter.NodeConfig.SetDefault("pruning.enabled", false)

	// "Amount of milestone transactions to keep in the database"
	parameter.NodeConfig.SetDefault("pruning.delay", 40000)

	// "Path to the ledger state file for your private tangle"
	parameter.NodeConfig.SetDefault("privateTangle.ledgerStatePath", "balances.txt")
}
