package utxo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestConfirmationApplyAndRollbackToEmptyLedger(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputNFT),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
	}

	spents := Spents{
		randomSpent(outputs[3]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	var outputCount int
	require.NoError(t, utxo.ForEachOutput(func(_ *Output) bool {
		outputCount++
		return true
	}))
	require.Equal(t, 5, outputCount)

	var unspentCount int
	require.NoError(t, utxo.ForEachUnspentOutput(func(_ *Output) bool {
		unspentCount++
		return true
	}))
	require.Equal(t, 3, unspentCount)

	var spentCount int
	require.NoError(t, utxo.ForEachSpentOutput(func(_ *Spent) bool {
		spentCount++
		return true
	}))
	require.Equal(t, 2, spentCount)

	require.NoError(t, utxo.RollbackConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	require.NoError(t, utxo.ForEachOutput(func(_ *Output) bool {
		require.Fail(t, "should not be called")
		return true
	}))

	require.NoError(t, utxo.ForEachUnspentOutput(func(_ *Output) bool {
		require.Fail(t, "should not be called")
		return true
	}))

	require.NoError(t, utxo.ForEachSpentOutput(func(_ *Spent) bool {
		require.Fail(t, "should not be called")
		return true
	}))
}

func TestConfirmationApplyAndRollbackToPreviousLedger(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	previousOutputs := Outputs{
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputNFT),
	}

	previousSpents := Spents{
		randomSpent(previousOutputs[1]),
	}

	previousMsIndex := milestone.Index(48)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(previousMsIndex, previousOutputs, previousSpents, nil, nil))

	ledgerIndex, err := utxo.ReadLedgerIndex()
	require.NoError(t, err)
	require.Equal(t, previousMsIndex, ledgerIndex)

	outputs := Outputs{
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
	}

	spents := Spents{
		randomSpent(previousOutputs[2]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(49)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	ledgerIndex, err = utxo.ReadLedgerIndex()
	require.NoError(t, err)
	require.Equal(t, msIndex, ledgerIndex)

	// Prepare values to check
	outputByOutputID := make(map[string]struct{})
	unspentByOutputID := make(map[string]struct{})
	for _, output := range previousOutputs {
		outputByOutputID[string(output.OutputID()[:])] = struct{}{}
		unspentByOutputID[string(output.OutputID()[:])] = struct{}{}
	}
	for _, output := range outputs {
		outputByOutputID[string(output.OutputID()[:])] = struct{}{}
		unspentByOutputID[string(output.OutputID()[:])] = struct{}{}
	}

	spentByOutputID := make(map[string]struct{})
	for _, spent := range previousSpents {
		spentByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentByOutputID, string(spent.OutputID()[:]))
	}
	for _, spent := range spents {
		spentByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentByOutputID, string(spent.OutputID()[:]))
	}

	var outputCount int
	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		outputCount++
		_, has := outputByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(outputByOutputID, string(output.OutputID()[:]))
		return true
	}))
	require.Empty(t, outputByOutputID)
	require.Equal(t, 7, outputCount)

	var unspentCount int
	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		unspentCount++
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}))
	require.Empty(t, unspentByOutputID)
	require.Equal(t, 4, unspentCount)

	var spentCount int
	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		spentCount++
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}))
	require.Empty(t, spentByOutputID)
	require.Equal(t, 3, spentCount)

	require.NoError(t, utxo.RollbackConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	ledgerIndex, err = utxo.ReadLedgerIndex()
	require.NoError(t, err)
	require.Equal(t, previousMsIndex, ledgerIndex)

	// Prepare values to check
	outputByOutputID = make(map[string]struct{})
	unspentByOutputID = make(map[string]struct{})
	spentByOutputID = make(map[string]struct{})

	for _, output := range previousOutputs {
		outputByOutputID[string(output.OutputID()[:])] = struct{}{}
		unspentByOutputID[string(output.OutputID()[:])] = struct{}{}
	}

	for _, spent := range previousSpents {
		spentByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentByOutputID, string(spent.OutputID()[:]))
	}

	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		_, has := outputByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(outputByOutputID, string(output.OutputID()[:]))
		return true
	}))
	require.Empty(t, outputByOutputID)

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}))
	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}))
	require.Empty(t, spentByOutputID)
}
