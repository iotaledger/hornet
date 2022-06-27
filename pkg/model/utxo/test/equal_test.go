package utxo_test

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/pkg/model/utxo"
)

func EqualOutput(t *testing.T, expected *utxo.Output, actual *utxo.Output) {
	require.Equal(t, expected.OutputID(), actual.OutputID())
	require.Equal(t, expected.BlockID(), actual.BlockID())
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
	require.Equal(t, expected.OutputType(), actual.OutputType())
	require.Equal(t, expected.Deposit(), actual.Deposit())
	require.EqualValues(t, expected.Output(), actual.Output())
}

func EqualSpent(t *testing.T, expected *utxo.Spent, actual *utxo.Spent) {
	require.Equal(t, expected.OutputID(), actual.OutputID())
	require.Equal(t, expected.TargetTransactionID(), actual.TargetTransactionID())
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
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
