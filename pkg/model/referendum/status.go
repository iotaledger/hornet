package referendum

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
)

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
	Status         string            `json:"status"`
	Questions      []*QuestionStatus `json:"questions,omitempty"`
}

func (rm *ReferendumManager) ReferendumStatus(referendumID ReferendumID) (*ReferendumStatus, error) {

	confirmedMilestoneIndex := rm.syncManager.ConfirmedMilestoneIndex()

	referendum := rm.Referendum(referendumID)
	if referendum == nil {
		return nil, ErrReferendumNotFound
	}

	status := &ReferendumStatus{
		MilestoneIndex: confirmedMilestoneIndex,
		Status:         referendum.Status(confirmedMilestoneIndex),
	}

	// For each referendum, iterate over all questions
	for idx, question := range referendum.BallotQuestions() {
		questionIndex := uint8(idx)

		questionStatus := &QuestionStatus{}
		// For each question, iterate over all answers. Include 0 here, since that is valid, i.e. answer skipped by voter
		for idx := 0; idx <= len(question.Answers); idx++ {
			answerIndex := uint8(idx)

			currentBalance, err := rm.CurrentVoteBalanceForQuestionAndAnswer(referendumID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}

			accumulatedBalance, err := rm.AccumulatedVoteBalanceForQuestionAndAnswer(referendumID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}
			questionStatus.Answers = append(questionStatus.Answers, &AnswerStatus{
				Index:       answerIndex,
				Current:     currentBalance,
				Accumulated: accumulatedBalance,
			})
		}
		status.Questions = append(status.Questions, questionStatus)
	}

	return status, nil
}
