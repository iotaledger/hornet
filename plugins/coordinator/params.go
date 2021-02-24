package coordinator

import (
	"runtime"

	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// the path to the state file of the coordinator
	CfgCoordinatorStateFilePath = "coordinator.stateFilePath"
	// the interval milestones are issued
	CfgCoordinatorIntervalSeconds = "coordinator.intervalSeconds"
	// the signing provider the coordinator uses to sign a milestone (local/remote)
	CfgCoordinatorSigningProvider = "coordinator.signing.provider"
	// the address of the remote signing provider (insecure connection!)
	CfgCoordinatorSigningRemoteAddress = "coordinator.signing.remoteAddress"
	// the amount of workers used for calculating PoW when issuing checkpoints and milestones
	CfgCoordinatorPoWWorkerCount = "coordinator.powWorkerCount"
	// the maximum amount of known messages for milestone tipselection
	// if this limit is exceeded, a new checkpoint is issued
	CfgCoordinatorCheckpointsMaxTrackedMessages = "coordinator.checkpoints.maxTrackedMessages"
	// the minimum threshold of unreferenced messages in the heaviest branch for milestone tipselection
	// if the value falls below that threshold, no more heaviest branch tips are picked
	CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold = "coordinator.tipsel.minHeaviestBranchUnreferencedMessagesThreshold"
	// the maximum amount of checkpoint messages with heaviest branch tips that are picked
	// if the heaviest branch is not below "UnreferencedMessagesThreshold" before
	CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint = "coordinator.tipsel.maxHeaviestBranchTipsPerCheckpoint"
	// the amount of checkpoint messages with random tips that are picked if a checkpoint is issued and at least
	// one heaviest branch tip was found, otherwise no random tips will be picked
	CfgCoordinatorTipselectRandomTipsPerCheckpoint = "coordinator.tipsel.randomTipsPerCheckpoint"
	// the maximum duration to select the heaviest branch tips in milliseconds
	CfgCoordinatorTipselectHeaviestBranchSelectionTimeoutMilliseconds = "coordinator.tipsel.heaviestBranchSelectionTimeoutMilliseconds"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgCoordinatorStateFilePath, "coordinator.state", "the path to the state file of the coordinator")
			fs.Int(CfgCoordinatorIntervalSeconds, 10, "the interval milestones are issued")
			fs.String(CfgCoordinatorSigningProvider, "local", "the signing provider the coordinator uses to sign a milestone (local/remote)")
			fs.String(CfgCoordinatorSigningRemoteAddress, "localhost:12345", "the address of the remote signing provider (insecure connection!)")
			fs.Int(CfgCoordinatorPoWWorkerCount, runtime.NumCPU()-1, "the amount of workers used for calculating PoW when issuing checkpoints and milestones")
			fs.Int(CfgCoordinatorCheckpointsMaxTrackedMessages, 10000, "maximum amount of known messages for milestone tipselection")
			fs.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold, 20, "minimum threshold of unreferenced messages in the heaviest branch")
			fs.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint, 10, "maximum amount of checkpoint messages with heaviest branch tips")
			fs.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint, 3, "amount of checkpoint messages with random tips")
			fs.Int(CfgCoordinatorTipselectHeaviestBranchSelectionTimeoutMilliseconds, 100, "the maximum duration to select the heaviest branch tips in milliseconds")
			return fs
		}(),
	},
	Masked: nil,
}
