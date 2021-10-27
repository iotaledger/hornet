package referendum

const (
	ReferendumStoreKeyPrefixReferendums byte = 0

	ReferendumStoreKeyPrefixMessages     byte = 1
	ReferendumStoreKeyPrefixOutputs      byte = 2
	ReferendumStoreKeyPrefixSpentOutputs byte = 3

	ReferendumStoreKeyPrefixCurrentVoteBalanceForQuestionAndAnswer byte = 4
	ReferendumStoreKeyPrefixTotalVoteBalanceForQuestionAndAnswer   byte = 5
)
