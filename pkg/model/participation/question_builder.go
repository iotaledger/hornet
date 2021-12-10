package participation

import (
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// NewQuestionBuilder creates a new QuestionBuilder.
func NewQuestionBuilder(text string, additionalInfo string) *QuestionBuilder {
	return &QuestionBuilder{
		q: &Question{
			Text:           text,
			Answers:        serializer.Serializables{},
			AdditionalInfo: additionalInfo,
		},
	}
}

// QuestionBuilder is used to easily build up a Question.
type QuestionBuilder struct {
	q *Question
}

// AddAnswer adds the given answer to the question.
func (qb *QuestionBuilder) AddAnswer(entry *Answer) *QuestionBuilder {
	qb.q.Answers = append(qb.q.Answers, entry)
	return qb
}

// Build builds the Question.
func (qb *QuestionBuilder) Build() (*Question, error) {
	if _, err := qb.q.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build question: %w", err)
	}
	return qb.q, nil
}
