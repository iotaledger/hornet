package coordinator

import (
	"runtime"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the path to the state file of the coordinator
	CfgCoordinatorStateFilePath = "coordinator.stateFilePath"
	// the interval milestones are issued
	CfgCoordinatorInterval = "coordinator.interval"
	// the signing provider the coordinator uses to sign a milestone (local/remote)
	CfgCoordinatorSigningProvider = "coordinator.signing.provider"
	// the address of the remote signing provider (insecure connection!)
	CfgCoordinatorSigningRemoteAddress = "coordinator.signing.remoteAddress"
	// the amount of workers used for calculating PoW when issuing checkpoints and milestones
	CfgCoordinatorPoWWorkerCount = "coordinator.powWorkerCount"
	// whether the coordinator quorum is enabled
	CfgCoordinatorQuorumEnabled = "coordinator.quorum.enabled"
	// the quorum groups used to ask other nodes for correct ledger state of the coordinator
	CfgCoordinatorQuorumGroups = "coordinator.quorum.groups"
	// the timeout until a node in the quorum must have answered
	CfgCoordinatorQuorumTimeout = "coordinator.quorum.timeout"
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
	// the maximum duration to select the heaviest branch tips
	CfgCoordinatorTipselectHeaviestBranchSelectionTimeout = "coordinator.tipsel.heaviestBranchSelectionTimeout"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgCoordinatorStateFilePath, "coordinator.state", "the path to the state file of the coordinator")
			fs.Duration(CfgCoordinatorInterval, 10*time.Second, "the interval milestones are issued")
			fs.String(CfgCoordinatorSigningProvider, "local", "the signing provider the coordinator uses to sign a milestone (local/remote)")
			fs.String(CfgCoordinatorSigningRemoteAddress, "localhost:12345", "the address of the remote signing provider (insecure connection!)")
			fs.Int(CfgCoordinatorPoWWorkerCount, runtime.NumCPU()-1, "the amount of workers used for calculating PoW when issuing checkpoints and milestones")
			fs.Bool(CfgCoordinatorQuorumEnabled, false, "whether the coordinator quorum is enabled")
			fs.Duration(CfgCoordinatorQuorumTimeout, 2*time.Second, "the timeout until a node in the quorum must have answered")
			fs.Int(CfgCoordinatorCheckpointsMaxTrackedMessages, 10000, "maximum amount of known messages for milestone tipselection")
			fs.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold, 20, "minimum threshold of unreferenced messages in the heaviest branch")
			fs.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint, 10, "maximum amount of checkpoint messages with heaviest branch tips")
			fs.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint, 3, "amount of checkpoint messages with random tips")
			fs.Duration(CfgCoordinatorTipselectHeaviestBranchSelectionTimeout, 100*time.Millisecond, "the maximum duration to select the heaviest branch tips")
			return fs
		}(),
	},
	Masked: nil,
}
