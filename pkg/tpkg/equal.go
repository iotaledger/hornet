//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package tpkg

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

func EqualOutput(t *testing.T, expected *utxo.Output, actual *utxo.Output) {
	require.Equal(t, expected.OutputID(), actual.OutputID())
	require.Equal(t, expected.BlockID(), actual.BlockID())
	require.Equal(t, expected.MilestoneIndexBooked(), actual.MilestoneIndexBooked())
	require.Equal(t, expected.OutputType(), actual.OutputType())

	var expectedIdent iotago.Address
	switch output := expected.Output().(type) {
	case iotago.TransIndepIdentOutput:
		expectedIdent = output.Ident()
	case iotago.TransDepIdentOutput:
		expectedIdent = output.Chain().ToAddress()
	default:
		require.Fail(t, "unsupported output type")
	}

	var actualIdent iotago.Address
	switch output := actual.Output().(type) {
	case iotago.TransIndepIdentOutput:
		actualIdent = output.Ident()
	case iotago.TransDepIdentOutput:
		actualIdent = output.Chain().ToAddress()
	default:
		require.Fail(t, "unsupported output type")
	}

	require.NotNil(t, expectedIdent)
	require.NotNil(t, actualIdent)
	require.True(t, expectedIdent.Equal(actualIdent))
	require.Equal(t, expected.Deposit(), actual.Deposit())
	require.EqualValues(t, expected.Output(), actual.Output())
}

func EqualSpent(t *testing.T, expected *utxo.Spent, actual *utxo.Spent) {
	require.Equal(t, expected.OutputID(), actual.OutputID())
	require.Equal(t, expected.TransactionIDSpent(), actual.TransactionIDSpent())
	require.Equal(t, expected.MilestoneIndexSpent(), actual.MilestoneIndexSpent())
	require.Equal(t, expected.MilestoneTimestampSpent(), actual.MilestoneTimestampSpent())
	EqualOutput(t, expected.Output(), actual.Output())
}

func EqualOutputs(t *testing.T, expected utxo.Outputs, actual utxo.Outputs) {
	require.Equal(t, len(expected), len(actual))

	// Sort Outputs by output ID.
	sort.Slice(expected, func(i, j int) bool {
		iOutputID := expected[i].OutputID()
		jOutputID := expected[j].OutputID()

		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		iOutputID := actual[i].OutputID()
		jOutputID := actual[j].OutputID()

		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualOutput(t, expected[i], actual[i])
	}
}

func EqualSpents(t *testing.T, expected utxo.Spents, actual utxo.Spents) {
	require.Equal(t, len(expected), len(actual))

	// Sort Spents by output ID.
	sort.Slice(expected, func(i, j int) bool {
		iOutputID := expected[i].OutputID()
		jOutputID := expected[j].OutputID()

		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		iOutputID := actual[i].OutputID()
		jOutputID := actual[j].OutputID()

		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualSpent(t, expected[i], actual[i])
	}
}
