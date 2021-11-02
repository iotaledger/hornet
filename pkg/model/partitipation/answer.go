package partitipation

import (
	"encoding/json"
	"fmt"

	"github.com/iotaledger/hive.go/serializer"
)

const (
	AnswerTextMaxLength           = 255
	AnswerAdditionalInfoMaxLength = 500
)

// Answer
type Answer struct {
	Index          uint8
	Text           string
	AdditionalInfo string
}

func (a *Answer) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode) (int, error) {
	return serializer.NewDeserializer(data).
		ReadNum(&a.Index, func(err error) error {
			return fmt.Errorf("unable to deserialize partitipation answer index: %w", err)
		}).
		ReadString(&a.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize partitipation answer text: %w", err)
		}, AnswerTextMaxLength).
		ReadString(&a.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize partitipation answer additional info: %w", err)
		}, AnswerAdditionalInfoMaxLength).
		Done()
}

func (a *Answer) Serialize(deSeriMode serializer.DeSerializationMode) ([]byte, error) {
	//TODO: validate text lengths
	return serializer.NewSerializer().
		WriteNum(a.Index, func(err error) error {
			return fmt.Errorf("unable to serialize partitipation answer index: %w", err)
		}).
		WriteString(a.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize partitipation answer text: %w", err)
		}).
		WriteString(a.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize partitipation answer additional info: %w", err)
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
