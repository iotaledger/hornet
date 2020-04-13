package config

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
	// start index of the first milestone
	CfgCoordinatorStartIndex = "coordinator.startIndex"
	// the interval milestones are issued
	CfgCoordinatorIntervalSeconds = "coordinator.intervalSeconds"
	// the amount of checkpoints issued between two milestones
	CfgCoordinatorCheckpointTransactions = "coordinator.checkpointTransactions"
)

func init() {
	NodeConfig.SetDefault(CfgCoordinatorAddress, "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9")
	NodeConfig.SetDefault(CfgCoordinatorSecurityLevel, 2)
	NodeConfig.SetDefault(CfgCoordinatorMerkleTreeDepth, 23)
	NodeConfig.SetDefault(CfgCoordinatorMWM, 14)
	NodeConfig.SetDefault(CfgCoordinatorStartIndex, 0)
	NodeConfig.SetDefault(CfgCoordinatorIntervalSeconds, 60)
	NodeConfig.SetDefault(CfgCoordinatorCheckpointTransactions, 5)
}
