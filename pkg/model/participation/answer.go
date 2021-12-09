package participation

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer"
)

const (
	AnswerTextMaxLength           = 255
	AnswerAdditionalInfoMaxLength = 500

	AnswerValueSkipped = 0
	AnswerValueInvalid = 255
)

var (
	ErrSerializationReservedValue = errors.New("reserved value used")
)

// Answer is a possible answer to a Ballot Question
type Answer struct {
	// Value is the value that should be used to pick this answer. It must be unique for each answer in a given question. Reserved values are 0 and 255.
	Value uint8
	// Text is the text of the answer.
	Text string
	// AdditionalInfo is an additional description text about the answer.
	AdditionalInfo string
}

func (a *Answer) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadNum(&a.Value, func(err error) error {
			return fmt.Errorf("unable to deserialize participation answer index: %w", err)
		}).
		ReadString(&a.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize participation answer text: %w", err)
		}, AnswerTextMaxLength).
		ReadString(&a.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize participation answer additional info: %w", err)
		}, AnswerAdditionalInfoMaxLength).
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if a.Value == AnswerValueSkipped || a.Value == AnswerValueInvalid {
					return fmt.Errorf("%w: answer is using a reserved value %d", ErrSerializationReservedValue, a.Value)
				}
			}
			return nil
		}).
		Done()
}

func (a *Answer) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	return serializer.NewSerializer().
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if len(a.Text) > AnswerTextMaxLength {
					return fmt.Errorf("%w: text too long. Max allowed %d", ErrSerializationStringLengthInvalid, AnswerTextMaxLength)
				}
				if len(a.AdditionalInfo) > AnswerAdditionalInfoMaxLength {
					return fmt.Errorf("%w: additional info too long. Max allowed %d", ErrSerializationStringLengthInvalid, AnswerAdditionalInfoMaxLength)
				}
				if a.Value == AnswerValueSkipped || a.Value == AnswerValueInvalid {
					return fmt.Errorf("%w: answer is using a reserved value %d", ErrSerializationReservedValue, a.Value)
				}
			}
			return nil
		}).
		WriteNum(a.Value, func(err error) error {
			return fmt.Errorf("unable to serialize participation answer index: %w", err)
		}).
		WriteString(a.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize participation answer text: %w", err)
		}).
		WriteString(a.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize participation answer additional info: %w", err)
		}).
		Serialize()
}

func (a *Answer) MarshalJSON() ([]byte, error) {
	jAnswer := &jsonAnswer{
		Value:          a.Value,
		Text:           a.Text,
		AdditionalInfo: a.AdditionalInfo,
	}
	return json.Marshal(jAnswer)
}

func (a *Answer) UnmarshalJSON(bytes []byte) error {
	jAnswer := &jsonAnswer{}
	if err := json.Unmarshal(bytes, jAnswer); err != nil {
		return err
	}
	seri, err := jAnswer.ToSerializable()
	if err != nil {
		return err
	}
	*a = *seri.(*Answer)
	return nil
}

// jsonAnswer defines the json representation of an Answer
type jsonAnswer struct {
	// Value is the value that should be used to pick this answer. It must be unique for each answer in a given question. Reserved values are 0 and 255.
	Value uint8 `json:"value"`
	// Text is the text of the answer.
	Text string `json:"text"`
	// AdditionalInfo is an additional description text about the answer.
	AdditionalInfo string `json:"additionalInfo"`
}

func (j *jsonAnswer) ToSerializable() (serializer.Serializable, error) {
	payload := &Answer{
		Value:          j.Value,
		Text:           j.Text,
		AdditionalInfo: j.AdditionalInfo,
	}
	return payload, nil
}
