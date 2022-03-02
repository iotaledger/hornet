package participation_test

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
)

func RandValidStaking() (*participation.Staking, []byte) {
	return RandValidStakingWithNumeratorDenominator(uint32(1+rand.Int31n(10000)), uint32(1+rand.Int31n(10000)))
}

func RandValidStakingWithNumeratorDenominator(numerator uint32, denominator uint32) (*participation.Staking, []byte) {
	return RandStaking(
		participation.StakingTextMaxLength,
		participation.StakingSymbolMaxLength,
		numerator,
		denominator,
		participation.StakingAdditionalInfoMaxLength,
	)
}

func RandStaking(textLen int, symbolLen int, numerator uint32, denominator uint32, additionalInfoLen int) (*participation.Staking, []byte) {
	s := &participation.Staking{
		Text:                   RandString(textLen),
		Symbol:                 RandString(symbolLen),
		Numerator:              numerator,
		Denominator:            denominator,
		RequiredMinimumRewards: rand.Uint64(),
		AdditionalInfo:         RandString(additionalInfoLen),
	}

	ms := marshalutil.New()
	ms.WriteUint32(participation.StakingPayloadTypeID)
	ms.WriteUint8(uint8(len(s.Text)))
	ms.WriteBytes([]byte(s.Text))
	ms.WriteUint8(uint8(len(s.Symbol)))
	ms.WriteBytes([]byte(s.Symbol))
	ms.WriteUint32(s.Numerator)
	ms.WriteUint32(s.Denominator)
	ms.WriteUint64(s.RequiredMinimumRewards)
	ms.WriteUint16(uint16(len(s.AdditionalInfo)))
	ms.WriteBytes([]byte(s.AdditionalInfo))

	return s, ms.Bytes()
}

func TestStaking_Deserialize(t *testing.T) {
	staking, stakingData := RandValidStaking()
	shortSymbol, shortSymbolData := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMinLength-1, 1000, 1000, participation.StakingAdditionalInfoMaxLength)
	longSymbol, longSymbolData := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMaxLength+1, 1000, 1000, participation.StakingAdditionalInfoMaxLength)
	longAdditionalInfo, longAdditionalInfoData := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMaxLength, 1000, 1000, participation.StakingAdditionalInfoMaxLength+1)
	invalidNumerator, invalidNumeratorData := RandValidStakingWithNumeratorDenominator(0, 1000)
	invalidDenominator, invalidDenominatorData := RandValidStakingWithNumeratorDenominator(1000, 0)

	tests := []struct {
		name   string
		data   []byte
		target *participation.Staking
		err    error
	}{
		{"ok", stakingData, staking, nil},
		{"not enough data", stakingData[:len(stakingData)-1], staking, serializer.ErrDeserializationNotEnoughData},
		{"too short symbol", shortSymbolData, shortSymbol, serializer.ErrDeserializationLengthInvalid},
		{"too long symbol", longSymbolData, longSymbol, serializer.ErrDeserializationLengthInvalid},
		{"too long additional info", longAdditionalInfoData, longAdditionalInfo, serializer.ErrDeserializationLengthInvalid},
		{"invalid numerator", invalidNumeratorData, invalidNumerator, participation.ErrInvalidNumeratorOrDenominator},
		{"invalid denominator", invalidDenominatorData, invalidDenominator, participation.ErrInvalidNumeratorOrDenominator},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &participation.Staking{}
			bytesRead, err := u.Deserialize(tt.data, serializer.DeSeriModePerformValidation, nil)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err))
				return
			}
			assert.Equal(t, len(tt.data), bytesRead)
			assert.EqualValues(t, tt.target, u)
		})
	}
}

func TestStaking_Serialize(t *testing.T) {
	staking, stakingData := RandValidStaking()
	longName, longNameData := RandStaking(participation.StakingTextMaxLength+1, participation.StakingSymbolMaxLength, 1000, 1000, participation.StakingAdditionalInfoMaxLength)
	shortSymbol, shortSymbolData := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMinLength-1, 1000, 1000, participation.StakingAdditionalInfoMaxLength)
	longSymbol, longSymbolData := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMaxLength+1, 1000, 1000, participation.StakingAdditionalInfoMaxLength)
	longAdditionalInfo, longAdditionalInfoData := RandStaking(participation.StakingTextMaxLength, participation.StakingSymbolMaxLength, 1000, 1000, participation.StakingAdditionalInfoMaxLength+1)
	invalidNumerator, invalidNumeratorData := RandValidStakingWithNumeratorDenominator(0, 1000)
	invalidDenominator, invalidDenominatorData := RandValidStakingWithNumeratorDenominator(1000, 0)

	tests := []struct {
		name   string
		source *participation.Staking
		target []byte
		err    error
	}{
		{"ok", staking, stakingData, nil},
		{"too long text", longName, longNameData, serializer.ErrStringTooLong},
		{"too short symbol", shortSymbol, shortSymbolData, participation.ErrSerializationStringLengthInvalid},
		{"too long symbol", longSymbol, longSymbolData, serializer.ErrStringTooLong},
		{"too long additional info", longAdditionalInfo, longAdditionalInfoData, serializer.ErrStringTooLong},
		{"invalid numerator", invalidNumerator, invalidNumeratorData, participation.ErrInvalidNumeratorOrDenominator},
		{"invalid denominator", invalidDenominator, invalidDenominatorData, participation.ErrInvalidNumeratorOrDenominator},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.source.Serialize(serializer.DeSeriModePerformValidation, nil)
			if tt.err != nil {
				assert.True(t, errors.Is(err, tt.err))
				return
			}
			assert.EqualValues(t, tt.target, data)
		})
	}
}
