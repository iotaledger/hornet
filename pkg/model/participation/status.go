package participation

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

// ParticipationEventStatus holds the status of the event
type ParticipationEventStatus struct {
	MilestoneIndex milestone.Index   `json:"milestoneIndex"`
	Status         string            `json:"status"`
	Questions      []*QuestionStatus `json:"questions,omitempty"`
	//TODO: add hash of all QuestionStatus to make comparison easier
}

func (rm *ParticipationManager) ParticipationEventStatus(eventID EventID) (*ParticipationEventStatus, error) {

	confirmedMilestoneIndex := rm.syncManager.ConfirmedMilestoneIndex()

	event := rm.Event(eventID)
	if event == nil {
		return nil, ErrEventNotFound
	}

	status := &ParticipationEventStatus{
		MilestoneIndex: confirmedMilestoneIndex,
		Status:         event.Status(confirmedMilestoneIndex),
	}

	// For each participation, iterate over all questions
	for idx, question := range event.BallotQuestions() {
		questionIndex := uint8(idx)

		questionStatus := &QuestionStatus{}
		// For each question, iterate over all answers. Include 0 here, since that is valid, i.e. answer skipped by voter
		// TODO: count invalid votes? -> maybe mapped to 255
		for idx := 0; idx <= len(question.Answers); idx++ {
			answerIndex := uint8(idx)

			currentBalance, err := rm.CurrentBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}

			accumulatedBalance, err := rm.AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerIndex)
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
