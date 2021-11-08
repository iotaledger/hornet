package participation

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// AnswerStatus holds the current and accumulated vote for an answer.
type AnswerStatus struct {
	Value       uint8  `json:"value"`
	Current     uint64 `json:"current"`
	Accumulated uint64 `json:"accumulated"`
}

// QuestionStatus holds the answers for a question.
type QuestionStatus struct {
	Answers []*AnswerStatus `json:"answers"`
}

// EventStatus holds the status of the event
type EventStatus struct {
	MilestoneIndex milestone.Index   `json:"milestoneIndex"`
	Status         string            `json:"status"`
	Questions      []*QuestionStatus `json:"questions,omitempty"`
	// TODO: add hash of all QuestionStatus to make comparison easier
}

// EventStatus returns the EventStatus for an event with the given eventID.
func (pm *ParticipationManager) EventStatus(eventID EventID) (*EventStatus, error) {

	confirmedMilestoneIndex := pm.syncManager.ConfirmedMilestoneIndex()

	event := pm.Event(eventID)
	if event == nil {
		return nil, ErrEventNotFound
	}

	status := &EventStatus{
		MilestoneIndex: confirmedMilestoneIndex,
		Status:         event.Status(confirmedMilestoneIndex),
	}

	// For each participation, iterate over all questions
	for idx, question := range event.BallotQuestions() {
		questionIndex := uint8(idx)

		questionStatus := &QuestionStatus{}

		balanceForAnswerValue := func(answerValue uint8) (*AnswerStatus, error) {
			currentBalance, err := pm.CurrentBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerValue)
			if err != nil {
				return nil, err
			}

			accumulatedBalance, err := pm.AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, questionIndex, answerValue)
			if err != nil {
				return nil, err
			}
			return &AnswerStatus{
				Value:       answerValue,
				Current:     currentBalance,
				Accumulated: accumulatedBalance,
			}, nil
		}

		// Add valid answer values
		for _, answer := range question.QuestionAnswers() {
			status, err := balanceForAnswerValue(answer.Value)
			if err != nil {
				return nil, err
			}
			questionStatus.Answers = append(questionStatus.Answers, status)
		}

		// Add skipped (value == 0)
		skippedValue, err := balanceForAnswerValue(AnswerValueSkipped)
		if err != nil {
			return nil, err
		}
		questionStatus.Answers = append(questionStatus.Answers, skippedValue)

		// Add invalid (value == 255)
		invalidValue, err := balanceForAnswerValue(AnswerValueInvalid)
		if err != nil {
			return nil, err
		}
		questionStatus.Answers = append(questionStatus.Answers, invalidValue)

		status.Questions = append(status.Questions, questionStatus)
	}

	return status, nil
}

func (q *QuestionStatus) StatusForAnswerValue(answerValue uint8) *AnswerStatus {
	for _, a := range q.Answers {
		if a.Value == answerValue {
			return a
		}
	}
	return nil
}
