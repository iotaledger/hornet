package participation

const (
	// Holds the events
	ParticipationStoreKeyPrefixEvents byte = 0

	// Holds the messages containing participations
	ParticipationStoreKeyPrefixMessages byte = 1

	// Tracks all active and past participations
	ParticipationStoreKeyPrefixTrackedOutputs         byte = 2
	ParticipationStoreKeyPrefixTrackedSpentOutputs    byte = 3
	ParticipationStoreKeyPrefixTrackedOutputByAddress byte = 8

	// Voting
	ParticipationStoreKeyPrefixBallotCurrentVoteBalanceForQuestionAndAnswer     byte = 4
	ParticipationStoreKeyPrefixBallotAccululatedVoteBalanceForQuestionAndAnswer byte = 5

	// Staking
	ParticipationStoreKeyPrefixStakingAddress            byte = 6
	ParticipationStoreKeyPrefixStakingTotalParticipation byte = 7
	ParticipationStoreKeyPrefixStakingCurrentRewards     byte = 9
)
