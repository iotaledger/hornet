package referendum

import (
	"encoding/json"
	"fmt"

	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	AnswerTextMaxLength           = 255
	AnswerAdditionalInfoMaxLength = 500
)

// Answer
type Answer struct {
	Index          uint8  `json:"index"`
	Text           string `json:"text"`
	AdditionalInfo string `json:"additionalInfo"`
}

func (a *Answer) Deserialize(data []byte, deSeriMode iotago.DeSerializationMode) (int, error) {
	return iotago.NewDeserializer(data).
		ReadNum(&a.Index, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum answer index: %w", err)
		}).
		ReadString(&a.Text, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum answer text: %w", err)
		}, AnswerTextMaxLength).
		ReadString(&a.AdditionalInfo, iotago.SeriSliceLengthAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize referendum answer additional info: %w", err)
		}, AnswerAdditionalInfoMaxLength).
		Done()
}

func (a *Answer) Serialize(deSeriMode iotago.DeSerializationMode) ([]byte, error) {
	return iotago.NewSerializer().
		WriteNum(a.Index, func(err error) error {
			return fmt.Errorf("unable to serialize referendum answer index: %w", err)
		}).
		WriteString(a.Text, iotago.SeriSliceLengthAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize referendum answer text: %w", err)
		}).
		WriteString(a.AdditionalInfo, iotago.SeriSliceLengthAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize referendum answer additional info: %w", err)
		}).
		Serialize()
}

func (a *Answer) MarshalJSON() ([]byte, error) {
	return json.Marshal(a)
}

func (ra *Answer) UnmarshalJSON(bytes []byte) error {
	if err := json.Unmarshal(bytes, ra); err != nil {
		return err
	}
	return nil
}
