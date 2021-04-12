package common_test

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/common"
)

func TestSoftError_Error(t *testing.T) {

	var originError = errors.New("an error")

	aWrappedSoftErr := common.SoftError(fmt.Errorf("wrap me up softly: %w", originError))
	aWrappedCritErr := common.CriticalError(fmt.Errorf("wrap me up critically: %w", originError))

	require.EqualValues(t, errors.Unwrap(common.IsSoftError(aWrappedSoftErr)), originError)
	require.EqualValues(t, errors.Unwrap(common.IsCriticalError(aWrappedCritErr)), originError)
}
