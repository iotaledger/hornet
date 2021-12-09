package utxo

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func EqualOutput(t *testing.T, expected *Output, actual *Output) {
	require.Equal(t, expected.OutputID()[:], actual.OutputID()[:])
	require.Equal(t, expected.MessageID()[:], actual.MessageID()[:])
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
	require.Equal(t, expected.OutputType(), actual.OutputType())
	require.Equal(t, expected.Deposit(), actual.Deposit())
	require.EqualValues(t, expected.Output(), actual.Output())
}

func EqualSpent(t *testing.T, expected *Spent, actual *Spent) {
	require.Equal(t, expected.OutputID()[:], actual.OutputID()[:])
	require.Equal(t, expected.TargetTransactionID()[:], actual.TargetTransactionID()[:])
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
	EqualOutput(t, expected.Output(), actual.Output())
}

func EqualOutputs(t *testing.T, expected Outputs, actual Outputs) {
	require.Equal(t, len(expected), len(actual))

	// Sort Outputs by output ID.
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(expected[i].OutputID()[:], expected[j].OutputID()[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		return bytes.Compare(actual[i].OutputID()[:], actual[j].OutputID()[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualOutput(t, expected[i], actual[i])
	}
}

func EqualSpents(t *testing.T, expected Spents, actual Spents) {
	require.Equal(t, len(expected), len(actual))

	// Sort Spents by output ID.
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(expected[i].OutputID()[:], expected[j].OutputID()[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		return bytes.Compare(actual[i].OutputID()[:], actual[j].OutputID()[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualSpent(t, expected[i], actual[i])
	}
}
