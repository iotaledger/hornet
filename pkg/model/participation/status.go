package participation

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

// AnswerStatus holds the current and accumulated vote for an answer.
type AnswerStatus struct {
	// Value is the value that identifies this answer.
	Value uint8 `json:"value"`
	// Current is the current voting weight of the answer.
	Current uint64 `json:"current"`
	// Accumulated is the accumulated voting weight of the answer.
	Accumulated uint64 `json:"accumulated"`
}

// QuestionStatus holds the answers for a question.
type QuestionStatus struct {
	// Answers holds the status of the answers.
	Answers []*AnswerStatus `json:"answers"`
}

// StakingStatus holds the status of a staking.
type StakingStatus struct {
	// Staked is the currently staked amount of tokens.
	Staked uint64 `json:"staked"`
	// Rewarded is the total staking reward.
	Rewarded uint64 `json:"rewarded"`
	// Symbol is the symbol of the rewarded tokens.
	Symbol string `json:"symbol"`
}

// EventStatus holds the status of the event
type EventStatus struct {
	// MilestoneIndex is the milestone index the status was calculated for.
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
	// Status is the status of the event. Valid options are: "upcoming", "commencing", "holding" and "ended".
	Status string `json:"status"`
	// Questions holds the answer status of the different questions of the event.
	Questions []*QuestionStatus `json:"questions,omitempty"`
	// Staking is the staking status of the event.
	Staking *StakingStatus `json:"staking,omitempty"`
	// Checksum is the SHA256 checksum of all the question and answer status or the staking amount and rewards calculated for this MilestoneIndex.
	Checksum string `json:"checksum"`
}

// EventStatus returns the EventStatus for an event with the given eventID.
func (pm *ParticipationManager) EventStatus(eventID EventID, milestone ...milestone.Index) (*EventStatus, error) {
	event := pm.Event(eventID)
	if event == nil {
		return nil, ErrEventNotFound
	}

	index := pm.syncManager.ConfirmedMilestoneIndex()
	if len(milestone) > 0 {
		index = milestone[0]
	}

	if index > event.EndMilestoneIndex() {
		index = event.EndMilestoneIndex()
	}

	status := &EventStatus{
		MilestoneIndex: index,
		Status:         event.Status(index),
	}

	// compute the sha256 of all the question and answer status or the staking amount and rewards to easily compare
	statusHash := sha256.New()
	if _, err := statusHash.Write(eventID[:]); err != nil {
		return nil, err
	}
	if err := binary.Write(statusHash, binary.LittleEndian, index); err != nil {
		return nil, err
	}

	// For each participation, iterate over all questions
	for idx, question := range event.BallotQuestions() {
		questionIndex := uint8(idx)
		if err := binary.Write(statusHash, binary.LittleEndian, questionIndex); err != nil {
			return nil, err
		}

		questionStatus := &QuestionStatus{}

		balanceForAnswerValue := func(answerValue uint8) (*AnswerStatus, error) {
			currentBalance, err := pm.CurrentBallotVoteBalanceForQuestionAndAnswer(eventID, index, questionIndex, answerValue)
			if err != nil {
				return nil, err
			}

			accumulatedBalance, err := pm.AccumulatedBallotVoteBalanceForQuestionAndAnswer(eventID, index, questionIndex, answerValue)
			if err != nil {
				return nil, err
			}

			if err := binary.Write(statusHash, binary.LittleEndian, answerValue); err != nil {
				return nil, err
			}
			if err := binary.Write(statusHash, binary.LittleEndian, currentBalance); err != nil {
				return nil, err
			}
			if err := binary.Write(statusHash, binary.LittleEndian, accumulatedBalance); err != nil {
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

	staking := event.Staking()
	if staking != nil {
		total, err := pm.totalStakingParticipationForEvent(eventID, index)
		if err != nil {
			return nil, err
		}

		if err := binary.Write(statusHash, binary.LittleEndian, total.staked); err != nil {
			return nil, err
		}
		if err := binary.Write(statusHash, binary.LittleEndian, total.rewarded); err != nil {
			return nil, err
		}

		status.Staking = &StakingStatus{
			Staked:   total.staked,
			Rewarded: total.rewarded,
			Symbol:   staking.Symbol,
		}
	}

	status.Checksum = iotago.EncodeHex(statusHash.Sum(nil))
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
