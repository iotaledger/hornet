package config

const (
	// the address of the coordinator
	CfgMilestoneCoordinator = "milestones.coordinator"
	// the security level used in coordinator signatures
	CfgMilestoneCoordinatorSecurityLevel = "milestones.coordinatorSecurityLevel"
	// the depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message.
	CfgMilestoneNumberOfKeysInAMilestone = "milestones.numberOfKeysInAMilestone"
	// the minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash.
	// increasing this number by 1 will result in proof of work that is 3 times as hard.
	CfgProtocolMWM = "protocol.mwm"
)

func init() {
	NodeConfig.SetDefault(CfgMilestoneCoordinator, "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9")
	NodeConfig.SetDefault(CfgMilestoneCoordinatorSecurityLevel, 2)
	NodeConfig.SetDefault(CfgMilestoneNumberOfKeysInAMilestone, 23)
	NodeConfig.SetDefault(CfgProtocolMWM, 14)
}
