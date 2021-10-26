package referendum

import "github.com/gohornet/hornet/pkg/model/milestone"

// AnswerStatus holds the current and accumulated vote for an answer.
type AnswerStatus struct {
	Index       uint8  `json:"index"`
	Current     uint64 `json:"current"`
	Accumulated uint64 `json:"accumulated"`
}

// QuestionStatus holds the answers for a question.
type QuestionStatus struct {
	Answers []*AnswerStatus `json:"answers"`
}

// ReferendumStatus holds the status for all questions
type ReferendumStatus struct {
	MilestoneIndex milestone.Index   `json:"milestoneIndex"`
	Questions      []*QuestionStatus `json:"questions"`
}

func (rm *ReferendumManager) ReferendumStatus(referendumID ReferendumID) (*ReferendumStatus, error) {

	confirmedMilestoneIndex := rm.syncManager.ConfirmedMilestoneIndex()

	referendum := rm.Referendum(referendumID)
	if referendum == nil {
		return nil, ErrReferendumNotFound
	}

	status := &ReferendumStatus{
		MilestoneIndex: confirmedMilestoneIndex,
	}

	// For each referendum, iterate over all questions
	for idx, value := range referendum.Questions {
		questionIndex := uint8(idx)
		question := value.(*Question) // force cast here since we are sure the stored Referendum is valid

		questionStatus := &QuestionStatus{}
		// For each question, iterate over all answers. Include 0 here, since that is valid, i.e. answer skipped by voter
		for idx := 0; idx <= len(question.Answers); idx++ {
			answerIndex := uint8(idx)

			currentBalance, err := rm.CurrentBalanceForReferendum(referendumID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}

			totalBalance, err := rm.TotalBalanceForReferendum(referendumID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}
			questionStatus.Answers = append(questionStatus.Answers, &AnswerStatus{
				Index:       answerIndex,
				Current:     currentBalance,
				Accumulated: totalBalance,
			})
		}
		status.Questions = append(status.Questions, questionStatus)
	}

	return status, nil
}
