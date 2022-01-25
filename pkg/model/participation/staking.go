package participation

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer/v2"
)

const (
	StakingPayloadTypeID uint32 = 1

	StakingTextMaxLength           = 255
	StakingSymbolMinLength         = 3
	StakingSymbolMaxLength         = 10
	StakingAdditionalInfoMaxLength = 500
)

var (
	ErrInvalidNumeratorOrDenominator = errors.New("numerator and denominator need to be greater than zero")
)

type Staking struct {
	// Text is the description text of the staking event.
	Text string
	// Symbol is the symbol of the rewarded tokens.
	Symbol string

	// Numerator is used in combination with Denominator to calculate the rewards per milestone per staked tokens.
	Numerator uint32
	// Denominator is used in combination with Numerator to calculate the rewards per milestone per staked tokens.
	Denominator uint32

	// RequiredMinimumRewards are the minimum rewards required to be included in the staking results.
	RequiredMinimumRewards uint64

	// AdditionalInfo is an additional description text about the staking event.
	AdditionalInfo string
}

func (s *Staking) Deserialize(data []byte, deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) (int, error) {
	return serializer.NewDeserializer(data).
		Skip(serializer.TypeDenotationByteSize, func(err error) error {
			return fmt.Errorf("unable to skip staking payload ID during deserialization: %w", err)
		}).
		ReadString(&s.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize staking text: %w", err)
		}, StakingTextMaxLength).
		ReadString(&s.Symbol, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to deserialize staking symbol text: %w", err)
		}, StakingSymbolMaxLength).
		ReadNum(&s.Numerator, func(err error) error {
			return fmt.Errorf("unable to deserialize staking numerator: %w", err)
		}).
		ReadNum(&s.Denominator, func(err error) error {
			return fmt.Errorf("unable to deserialize staking denominator: %w", err)
		}).
		ReadNum(&s.RequiredMinimumRewards, func(err error) error {
			return fmt.Errorf("unable to deserialize staking required minimum rewards: %w", err)
		}).
		ReadString(&s.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to deserialize staking additional info: %w", err)
		}, StakingAdditionalInfoMaxLength).
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if s.Numerator == 0 || s.Denominator == 0 {
					return ErrInvalidNumeratorOrDenominator
				}
				if len(s.Symbol) < StakingSymbolMinLength {
					return fmt.Errorf("%w: symbol length invalid. Min %d Max %d", serializer.ErrDeserializationLengthInvalid, StakingSymbolMinLength, StakingSymbolMaxLength)
				}
			}
			return nil
		}).
		Done()
}

func (s *Staking) Serialize(deSeriMode serializer.DeSerializationMode, deSeriCtx interface{}) ([]byte, error) {

	return serializer.NewSerializer().
		AbortIf(func(err error) error {
			if deSeriMode.HasMode(serializer.DeSeriModePerformValidation) {
				if s.Numerator == 0 || s.Denominator == 0 {
					return ErrInvalidNumeratorOrDenominator
				}
				if len(s.Symbol) < StakingSymbolMinLength {
					return fmt.Errorf("%w: symbol length invalid. Min %d Max %d", ErrSerializationStringLengthInvalid, StakingSymbolMinLength, StakingSymbolMaxLength)
				}
			}
			return nil
		}).
		WriteNum(StakingPayloadTypeID, func(err error) error {
			return fmt.Errorf("%w: unable to serialize staking payload ID", err)
		}).
		WriteString(s.Text, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize staking text: %w", err)
		}, StakingTextMaxLength).
		WriteString(s.Symbol, serializer.SeriLengthPrefixTypeAsByte, func(err error) error {
			return fmt.Errorf("unable to serialize staking symbol: %w", err)
		}, StakingSymbolMaxLength).
		WriteNum(s.Numerator, func(err error) error {
			return fmt.Errorf("unable to serialize staking numerator: %w", err)
		}).
		WriteNum(s.Denominator, func(err error) error {
			return fmt.Errorf("unable to serialize staking denominator: %w", err)
		}).
		WriteNum(s.RequiredMinimumRewards, func(err error) error {
			return fmt.Errorf("unable to serialize staking required minimum rewards: %w", err)
		}).
		WriteString(s.AdditionalInfo, serializer.SeriLengthPrefixTypeAsUint16, func(err error) error {
			return fmt.Errorf("unable to serialize staking additional info: %w", err)
		}, StakingAdditionalInfoMaxLength).
		Serialize()
}

func (s *Staking) MarshalJSON() ([]byte, error) {
	j := &jsonStaking{
		Type:                   int(StakingPayloadTypeID),
		Text:                   s.Text,
		Symbol:                 s.Symbol,
		Numerator:              s.Numerator,
		Denominator:            s.Denominator,
		RequiredMinimumRewards: s.RequiredMinimumRewards,
		AdditionalInfo:         s.AdditionalInfo,
	}
	return json.Marshal(j)
}

func (s *Staking) UnmarshalJSON(bytes []byte) error {
	j := &jsonStaking{
		Type: int(StakingPayloadTypeID),
	}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	seri, err := j.ToSerializable()
	if err != nil {
		return err
	}
	*s = *seri.(*Staking)
	return nil
}

// jsonStaking defines the json representation of a Staking.
type jsonStaking struct {
	// Type is the type of the event.
	Type int `json:"type"`
	// Text is the description text of the staking event.
	Text string `json:"text"`
	// Symbol is the symbol of the rewarded tokens.
	Symbol string `json:"symbol"`
	// Numerator is used in combination with Denominator to calculate the rewards per milestone per staked tokens.
	Numerator uint32 `json:"numerator"`
	// Denominator is used in combination with Numerator to calculate the rewards per milestone per staked tokens.
	Denominator uint32 `json:"denominator"`
	// RequiredMinimumRewards are the minimum rewards required to be included in the staking results.
	RequiredMinimumRewards uint64 `json:"requiredMinimumRewards"`
	// AdditionalInfo is an additional description text about the staking event.
	AdditionalInfo string `json:"additionalInfo"`
}

func (j *jsonStaking) ToSerializable() (serializer.Serializable, error) {
	payload := &Staking{
		Text:                   j.Text,
		Symbol:                 j.Symbol,
		Numerator:              j.Numerator,
		Denominator:            j.Denominator,
		RequiredMinimumRewards: j.RequiredMinimumRewards,
		AdditionalInfo:         j.AdditionalInfo,
	}
	return payload, nil
}
