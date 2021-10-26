package referendum

const (
	ReferendumStoreKeyPrefixReferendums byte = 0

	ReferendumStoreKeyPrefixMessages byte = 1
	ReferendumStoreKeyPrefixOutputs  byte = 2

	ReferendumStoreKeyPrefixCurrentVoteBalanceForQuestionAndAnswer byte = 3
	ReferendumStoreKeyPrefixTotalVoteBalanceForQuestionAndAnswer   byte = 4
)
