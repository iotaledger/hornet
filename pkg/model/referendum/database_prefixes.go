package referendum

const (
	ReferendumStoreKeyPrefixReferendums byte = 0

	ReferendumStoreKeyPrefixMessages byte = 1

	ReferendumStoreKeyPrefixReferendumOutputs      byte = 2
	ReferendumStoreKeyPrefixReferendumSpentOutputs byte = 3

	ReferendumStoreKeyPrefixCurrentVoteBalanceForQuestionAndAnswer byte = 4
	ReferendumStoreKeyPrefixTotalVoteBalanceForQuestionAndAnswer   byte = 5
)
