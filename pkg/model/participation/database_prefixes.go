package participation

const (
	// Holds the events
	ParticipationStoreKeyPrefixEvents byte = 0

	// Holds the messages containing participations
	ParticipationStoreKeyPrefixMessages byte = 1

	// Tracks all active and past participations
	ParticipationStoreKeyPrefixReferendumOutputs      byte = 2
	ParticipationStoreKeyPrefixReferendumSpentOutputs byte = 3

	// Voting
	ParticipationStoreKeyPrefixBallotCurrentVoteBalanceForQuestionAndAnswer     byte = 4
	ParticipationStoreKeyPrefixBallotAccululatedVoteBalanceForQuestionAndAnswer byte = 5
)
