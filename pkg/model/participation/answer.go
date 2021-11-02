package participation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

const (
	AnswerTextMaxLength           = 255
	AnswerAdditionalInfoMaxLength = 500

	AnswerSkippedIndex = 0
	AnswerInvalidIndex = 255
)

// Answer is a possible answer to a Ballot Question
type Answer struct {
	Index          uint8
	Text           string
	AdditionalInfo string
}

func (a *Answer) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadNum(&a.Index, func(err error) error {
			return fmt.Errorf("unable to deserialize participation answer index: %w", err)
		}).
		ReadString(&a.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize participation answer text: %w", err)
		}, AnswerTextMaxLength).
		ReadString(&a.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize participation answer additional info: %w", err)
		}, AnswerAdditionalInfoMaxLength).
		Done()
}

func (a *Answer) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	// TODO: validate text lengths

	// TODO: validate that answer index != 0 and != 255

	return serializer.NewSerializer().
		WriteNum(a.Index, func(err error) error {
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
		Index:          a.Index,
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
	Index          uint8  `json:"index"`
	Text           string `json:"text"`
	AdditionalInfo string `json:"additionalInfo"`
}

func (j *jsonAnswer) ToSerializable() (serializer.Serializable, error) {
	payload := &Answer{
		Index:          j.Index,
		Text:           j.Text,
		AdditionalInfo: j.AdditionalInfo,
	}
	return payload, nil
}
