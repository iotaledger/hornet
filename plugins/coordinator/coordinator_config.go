package coordinator

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/core/cli"
	"github.com/gohornet/hornet/pkg/model/coordinator"
)

const (
	// the ed25519 public key of the coordinator in hex representation
	CfgCoordinatorPublicKeyRangesJSON = "publicKeyRanges"
	// the ed25519 public key of the coordinator in hex representation
	CfgCoordinatorPublicKeyRanges = "coordinator.publicKeyRanges"
	// the minimum PoW score required by the network
	CfgCoordinatorMinPoWScore = "coordinator.minPoWScore"
	// the path to the state file of the coordinator
	CfgCoordinatorStateFilePath = "coordinator.stateFilePath"
	// the interval milestones are issued
	CfgCoordinatorIntervalSeconds = "coordinator.intervalSeconds"
	// the amount of public keys in a milestone
	CfgCoordinatorMilestonePublicKeyCount = "coordinator.milestonePublicKeyCount"
	// the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)
	CfgCoordinatorMilestoneMerkleTreeHashFunc = "coordinator.milestoneMerkleTreeHashFunc"
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
	CfgCoordinatorTipselectHeaviestBranchSelectionDeadlineMilliseconds = "coordinator.tipsel.heaviestBranchSelectionDeadlineMilliseconds"
)

var (
	cooPubKeyRangesFlag = flag.String("publicKeyRanges", "", "overwrite public key ranges (JSON)")
)

func init() {
	// ToDo: values from IF
	if err := cli.Config.NodeConfig.SetDefault(CfgCoordinatorPublicKeyRanges, &coordinator.PublicKeyRanges{
		&coordinator.PublicKeyRange{Key: "ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
		&coordinator.PublicKeyRange{Key: "f1a319ff4e909c0ac9f2771d79feceed3c3bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
		&coordinator.PublicKeyRange{Key: "ced3c3f1a319ff4e909f2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 800, EndIndex: 1000},
	}); err != nil {
		panic(err)
	}

	cli.ConfigFlagSet.Float64(CfgCoordinatorMinPoWScore, 4000, "the minimum PoW score required by the network.")
	cli.ConfigFlagSet.String(CfgCoordinatorStateFilePath, "coordinator.state", "the path to the state file of the coordinator")
	cli.ConfigFlagSet.Int(CfgCoordinatorIntervalSeconds, 10, "the interval milestones are issued")
	cli.ConfigFlagSet.Int(CfgCoordinatorMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")
	cli.ConfigFlagSet.String(CfgCoordinatorMilestoneMerkleTreeHashFunc, "BLAKE2b-512", "the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)")
	cli.ConfigFlagSet.Int(CfgCoordinatorCheckpointsMaxTrackedMessages, 10000, "maximum amount of known messages for milestone tipselection")
	cli.ConfigFlagSet.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold, 20, "minimum threshold of unreferenced messages in the heaviest branch")
	cli.ConfigFlagSet.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint, 10, "maximum amount of checkpoint messages with heaviest branch tips")
	cli.ConfigFlagSet.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint, 3, "amount of checkpoint messages with random tips")
	cli.ConfigFlagSet.Int(CfgCoordinatorTipselectHeaviestBranchSelectionDeadlineMilliseconds, 100, "the maximum duration to select the heaviest branch tips in milliseconds")
}
