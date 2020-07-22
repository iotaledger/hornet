package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the address of the coordinator
	CfgCoordinatorAddress = "coordinator.address"
	// the security level used in coordinator signatures
	CfgCoordinatorSecurityLevel = "coordinator.securityLevel"
	// the depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message.
	CfgCoordinatorMerkleTreeDepth = "coordinator.merkleTreeDepth"
	// the minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash.
	// increasing this number by 1 will result in proof of work that is 3 times as hard.
	CfgCoordinatorMWM = "coordinator.mwm"
	// the path to the state file of the coordinator
	CfgCoordinatorStateFilePath = "coordinator.stateFilePath"
	// the path to the Merkle tree of the coordinator
	CfgCoordinatorMerkleTreeFilePath = "coordinator.merkleTreeFilePath"
	// the interval milestones are issued
	CfgCoordinatorIntervalSeconds = "coordinator.intervalSeconds"
	// the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)
	CfgCoordinatorMilestoneMerkleTreeHashFunc = "coordinator.milestoneMerkleTreeHashFunc"
	// the maximum amount of known approvees for milestone tipselection
	// if this limit is exceeded, a new checkpoint is issued
	CfgCoordinatorCheckpointsMaxApproveesCount = "coordinator.checkpoints.maxApproveesCount"
	// the maximum amount of known tips for milestone tipselection
	// if this limit is exceeded, a new checkpoint is issued
	CfgCoordinatorCheckpointsMaxTipsCount = "coordinator.checkpoints.maxTipsCount"
	// the minimum threshold of unconfirmed transactions in the heaviest branch for milestone tipselection
	// if the value falls below that threshold, no more heaviest branch tips are picked
	CfgCoordinatorTipselectMinHeaviestBranchUnconfirmedTransactionsThreshold = "coordinator.tipsel.minHeaviestBranchUnconfirmedTransactionsThreshold"
	// the maximum amount of checkpoint transactions with heaviest branch tips that are picked
	// if the heaviest branch is not below "UnconfirmedTransactionsThreshold" before
	CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint = "coordinator.tipsel.maxHeaviestBranchTipsPerCheckpoint"
	// the amount of checkpoint transactions with random tips that are picked if a checkpoint is issued and at least
	// one heaviest branch tip was found, otherwise no random tips will be picked
	CfgCoordinatorTipselectRandomTipsPerCheckpoint = "coordinator.tipsel.randomTipsPerCheckpoint"
)

func init() {
	flag.String(CfgCoordinatorAddress, "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9", "the address of the coordinator")
	flag.Int(CfgCoordinatorSecurityLevel, 2, "the security level used in coordinator signatures")
	flag.Int(CfgCoordinatorMerkleTreeDepth, 23, "the depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message.")
	flag.Int(CfgCoordinatorMWM, 14, "the minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash. "+
		"increasing this number by 1 will result in proof of work that is 3 times as hard.")
	flag.String(CfgCoordinatorStateFilePath, "coordinator.state", "the path to the state file of the coordinator")
	flag.String(CfgCoordinatorMerkleTreeFilePath, "coordinator.tree", "the path to the Merkle tree of the coordinator")
	flag.Int(CfgCoordinatorIntervalSeconds, 60, "the interval milestones are issued")
	flag.String(CfgCoordinatorMilestoneMerkleTreeHashFunc, "BLAKE2b-512", "the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)")
	flag.Int(CfgCoordinatorCheckpointsMaxApproveesCount, 10000, "maximum amount of known approvees for milestone tipselection")
	flag.Int(CfgCoordinatorCheckpointsMaxTipsCount, 100, "maximum amount of known tips for milestone tipselection")
	flag.Int(CfgCoordinatorTipselectMinHeaviestBranchUnconfirmedTransactionsThreshold, 3, "minimum threshold of unconfirmed transactions in the heaviest branch")
	flag.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint, 10, "maximum amount of checkpoint transactions with heaviest branch tips")
	flag.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint, 3, "amount of checkpoint transactions with random tips")
}
