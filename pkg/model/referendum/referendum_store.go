package referendum

func (rm *ReferendumManager) CurrentBalanceForReferendum(referendumID ReferendumID, questionIdx uint8, answerIdx uint8) (uint64, error) {

	return 0, nil
}

func (rm *ReferendumManager) TotalBalanceForReferendum(referendumID ReferendumID, questionIdx uint8, answerIdx uint8) (uint64, error) {

	return 0, nil
}

func setTotalBalanceForReferendum(referendumID ReferendumID, questionIdx uint8, answerIdx uint8, total uint64, mutations kvstore.BatchedMutations) error {

	return nil
}
