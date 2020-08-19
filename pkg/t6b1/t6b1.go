// Package ternary implements ternary encoding as specified by the IOTA Protocol RFC-0015.
// https://github.com/Wollac/iota-crypto-demo/blob/master/pkg/encoding/ternary/ternary.go
package t6b1

import (
	"fmt"
	"strings"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
)

func tryteToTryteValue(t byte) int8 {
	return trinary.TryteToTryteValueLUT[t-'9']
}

// BytesToTrytes converts a bit string into its ternary representation encoding one byte a two trytes.
func BytesToTrytes(bytes []byte) (trinary.Trytes, error) {
	if err := ValidBytesForTrytes(bytes); err != nil {
		return "", err
	}
	return MustBytesToTrytes(bytes), nil
}

// MustBytesToTrytes converts a bit string into its ternary representation encoding one byte a two trytes.
// The input is not validated before the computation.
func MustBytesToTrytes(bytes []byte) trinary.Trytes {
	var trytes strings.Builder
	trytes.Grow(len(bytes) * 2)

	for i := range bytes {
		// convert the signed byte value to two trytes.
		// this is equivalent to: IntToTrytes(int8(bytes[i]), 2)
		v := int(int8(bytes[i])) + (consts.TryteRadix/2)*consts.TryteRadix + consts.TryteRadix/2 // make un-balanced
		quo, rem := v/consts.TryteRadix, v%consts.TryteRadix
		trytes.WriteByte(trinary.TryteValueToTyteLUT[rem])
		trytes.WriteByte(trinary.TryteValueToTyteLUT[quo])
	}
	return trytes.String()
}

// ValidBytesForTrytes checks whether the given slice of bytes is a valid input for BytesToTrytes.
func ValidBytesForTrytes(bytes []byte) error {
	if len(bytes) == 0 {
		return consts.ErrInvalidBytesLength
	}
	return nil
}

// TrytesToBytes the ternary representation of a bit string back to its original slice of bytes.
func TrytesToBytes(trytes trinary.Trytes) ([]byte, error) {
	if err := ValidTrytesForBytes(trytes); err != nil {
		return nil, err
	}
	return MustTrytesToBytes(trytes), nil
}

// MustTrytesToBytes the ternary representation of a bit string back to its original slice of bytes.
// The input is not validated before the computation.
func MustTrytesToBytes(trytes trinary.Trytes) []byte {
	trytesLength := len(trytes)

	bytes := make([]byte, trytesLength/2)
	for i := 0; i < trytesLength; i += 2 {
		v := tryteToTryteValue(trytes[i]) + tryteToTryteValue(trytes[i+1])*consts.TryteRadix

		bytes[i/2] = byte(v)
	}
	return bytes
}

// ValidTrytesForBytes checks whether the given trytes are a valid input for TrytesToBytes.
func ValidTrytesForBytes(trytes trinary.Trytes) error {
	tryteLen := len(trytes)
	if tryteLen < 1 || tryteLen%2 != 0 {
		return fmt.Errorf("%w: length must be even", consts.ErrInvalidTrytes)
	}
	if err := trinary.ValidTrytes(trytes); err != nil {
		return err
	}
	for i := 0; i < tryteLen; i += 2 {
		v := int(tryteToTryteValue(trytes[i])) + int(tryteToTryteValue(trytes[i+1]))*consts.TryteRadix
		// the value must fit into an int8, i.e. -128 <= v <= 127
		if int(int8(v)) != v {
			return fmt.Errorf("%w: at index %d (trytes: %s)", consts.ErrInvalidTrytes, i, trytes[i:i+2])
		}
	}
	return nil
}
