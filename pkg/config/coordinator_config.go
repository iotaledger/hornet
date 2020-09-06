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
	// the path to the state file of the coordinator
	CfgCoordinatorStateFilePath = "coordinator.stateFilePath"
	// the path to the Merkle tree of the coordinator
	CfgCoordinatorMerkleTreeFilePath = "coordinator.merkleTreeFilePath"
	// the interval milestones are issued
	CfgCoordinatorIntervalSeconds = "coordinator.intervalSeconds"
	// the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)
	CfgCoordinatorMilestoneMerkleTreeHashFunc = "coordinator.milestoneMerkleTreeHashFunc"
	// the maximum amount of known bundle tails for milestone tipselection
	// if this limit is exceeded, a new checkpoint is issued
	CfgCoordinatorCheckpointsMaxTrackedTails = "coordinator.checkpoints.maxTrackedTransactions"
	// the minimum threshold of unconfirmed transactions in the heaviest branch for milestone tipselection
	// if the value falls below that threshold, no more heaviest branch tips are picked
	CfgCoordinatorTipselectMinHeaviestBranchUnconfirmedTransactionsThreshold = "coordinator.tipsel.minHeaviestBranchUnconfirmedTransactionsThreshold"
	// the maximum amount of checkpoint transactions with heaviest branch tips that are picked
	// if the heaviest branch is not below "UnconfirmedTransactionsThreshold" before
	CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint = "coordinator.tipsel.maxHeaviestBranchTipsPerCheckpoint"
	// the amount of checkpoint transactions with random tips that are picked if a checkpoint is issued and at least
	// one heaviest branch tip was found, otherwise no random tips will be picked
	CfgCoordinatorTipselectRandomTipsPerCheckpoint = "coordinator.tipsel.randomTipsPerCheckpoint"
	// the maximum duration to select the heaviest branch tips in milliseconds
	CfgCoordinatorTipselectHeaviestBranchSelectionDeadlineMilliseconds = "coordinator.tipsel.heaviestBranchSelectionDeadlineMilliseconds"
)

func init() {
	configFlagSet.String(CfgCoordinatorAddress, "UDYXTZBE9GZGPM9SSQV9LTZNDLJIZMPUVVXYXFYVBLIEUHLSEWFTKZZLXYRHHWVQV9MNNX9KZC9D9UZWZ", "the address of the coordinator")
	configFlagSet.Int(CfgCoordinatorSecurityLevel, 2, "the security level used in coordinator signatures")
	configFlagSet.Int(CfgCoordinatorMerkleTreeDepth, 24, "the depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message.")
	configFlagSet.Int(CfgCoordinatorMWM, 14, "the minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash. "+
		"increasing this number by 1 will result in proof of work that is 3 times as hard.")
	configFlagSet.String(CfgCoordinatorStateFilePath, "coordinator.state", "the path to the state file of the coordinator")
	configFlagSet.String(CfgCoordinatorMerkleTreeFilePath, "coordinator.tree", "the path to the Merkle tree of the coordinator")
	configFlagSet.Int(CfgCoordinatorIntervalSeconds, 10, "the interval milestones are issued")
	configFlagSet.String(CfgCoordinatorMilestoneMerkleTreeHashFunc, "BLAKE2b-512", "the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)")
	configFlagSet.Int(CfgCoordinatorCheckpointsMaxTrackedTails, 10000, "maximum amount of known bundle tails for milestone tipselection")
	configFlagSet.Int(CfgCoordinatorTipselectMinHeaviestBranchUnconfirmedTransactionsThreshold, 20, "minimum threshold of unconfirmed transactions in the heaviest branch")
	configFlagSet.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint, 10, "maximum amount of checkpoint transactions with heaviest branch tips")
	configFlagSet.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint, 3, "amount of checkpoint transactions with random tips")
	configFlagSet.Int(CfgCoordinatorTipselectHeaviestBranchSelectionDeadlineMilliseconds, 100, "the maximum duration to select the heaviest branch tips in milliseconds")
}
