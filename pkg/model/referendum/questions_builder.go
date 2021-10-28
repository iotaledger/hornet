package referendum

import (
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

// NewQuestionsBuilder creates a new QuestionsBuilder.
func NewQuestionsBuilder() *QuestionsBuilder {
	return &QuestionsBuilder{
		questions: &Questions{},
	}
}

// QuestionBuilder is used to easily build up a Question.
type QuestionsBuilder struct {
	questions *Questions
}

// AddQuestion adds the given question to the questions.
func (qb *QuestionsBuilder) AddQuestion(entry *Question) *QuestionsBuilder {
	qb.questions.Questions = append(qb.questions.Questions, entry)
	return qb
}

// Build builds the Question.
func (qb *QuestionsBuilder) Build() (*Questions, error) {
	if _, err := qb.questions.Serialize(serializer.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to build question: %w", err)
	}
	return qb.questions, nil
}
