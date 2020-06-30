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
	// the amount of checkpoints issued between two milestones
	CfgCoordinatorCheckpointTransactions = "coordinator.checkpointTransactions"
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
	flag.Int(CfgCoordinatorCheckpointTransactions, 5, "the amount of checkpoints issued between two milestones")
}
