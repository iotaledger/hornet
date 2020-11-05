package config

import (
	"encoding/json"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/model/milestone"
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

type PublicKeyRange struct {
	Key        string          `json:"key" koanf:"key"`
	StartIndex milestone.Index `json:"start" koanf:"start"`
	EndIndex   milestone.Index `json:"end" koanf:"end"`
}

type PublicKeyRanges []*PublicKeyRange

var (
	cooPubKeyRangesFlag = flag.String("publicKeyRanges", "", "overwrite public key ranges (JSON)")
)

func init() {
	// ToDo: values from IF
	if err := NodeConfig.SetDefault(CfgCoordinatorPublicKeyRanges, &PublicKeyRanges{
		&PublicKeyRange{Key: "ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
		&PublicKeyRange{Key: "f1a319ff4e909c0ac9f2771d79feceed3c3bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
		&PublicKeyRange{Key: "ced3c3f1a319ff4e909f2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 800, EndIndex: 1000},
	}); err != nil {
		panic(err)
	}

	configFlagSet.Int(CfgCoordinatorMinPoWScore, 4000, "the minimum PoW score required by the network.")
	configFlagSet.String(CfgCoordinatorStateFilePath, "coordinator.state", "the path to the state file of the coordinator")
	configFlagSet.Int(CfgCoordinatorIntervalSeconds, 10, "the interval milestones are issued")
	configFlagSet.Int(CfgCoordinatorMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")
	configFlagSet.String(CfgCoordinatorMilestoneMerkleTreeHashFunc, "BLAKE2b-512", "the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)")
	configFlagSet.Int(CfgCoordinatorCheckpointsMaxTrackedMessages, 10000, "maximum amount of known messages for milestone tipselection")
	configFlagSet.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold, 20, "minimum threshold of unreferenced messages in the heaviest branch")
	configFlagSet.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint, 10, "maximum amount of checkpoint messages with heaviest branch tips")
	configFlagSet.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint, 3, "amount of checkpoint messages with random tips")
	configFlagSet.Int(CfgCoordinatorTipselectHeaviestBranchSelectionDeadlineMilliseconds, 100, "the maximum duration to select the heaviest branch tips in milliseconds")
}

func CoordinatorPublicKeyRanges() PublicKeyRanges {
	r := PublicKeyRanges{}

	if *cooPubKeyRangesFlag != "" {
		// load from special CLI flag
		if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &r); err != nil {
			panic(err)
		}
	} else {
		// load from config or default value
		if err := NodeConfig.Unmarshal(CfgCoordinatorPublicKeyRanges, &r); err != nil {
			panic(err)
		}
	}

	return r
}
