package utxo

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestConfirmationApplyAndRollbackToEmptyLedger(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		RandUTXOOutput(iotago.OutputExtended),
		RandUTXOOutput(iotago.OutputExtended),
		RandUTXOOutput(iotago.OutputNFT),      // spent
		RandUTXOOutput(iotago.OutputExtended), // spent
		RandUTXOOutput(iotago.OutputAlias),
		RandUTXOOutput(iotago.OutputNFT),
		RandUTXOOutput(iotago.OutputFoundry),
	}

	msIndex := milestone.Index(756)
	msTimestamp := rand.Uint64()

	spents := Spents{
		RandUTXOSpent(outputs[3], msIndex, msTimestamp),
		RandUTXOSpent(outputs[2], msIndex, msTimestamp),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	var outputCount int
	require.NoError(t, utxo.ForEachOutput(func(_ *Output) bool {
		outputCount++
		return true
	}))
	require.Equal(t, 7, outputCount)

	var unspentCount int
	require.NoError(t, utxo.ForEachUnspentOutput(func(_ *Output) bool {
		unspentCount++
		return true
	}))
	require.Equal(t, 5, unspentCount)

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
		RandUTXOOutput(iotago.OutputExtended),
		RandUTXOOutput(iotago.OutputExtended), // spent
		RandUTXOOutput(iotago.OutputNFT),      // spent on 2nd confirmation
	}

	previousMsIndex := milestone.Index(48)
	previousMsTimestamp := rand.Uint64()
	previousSpents := Spents{
		RandUTXOSpent(previousOutputs[1], previousMsIndex, previousMsTimestamp),
	}
	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(previousMsIndex, previousOutputs, previousSpents, nil, nil))

	ledgerIndex, err := utxo.ReadLedgerIndex()
	require.NoError(t, err)
	require.Equal(t, previousMsIndex, ledgerIndex)

	outputs := Outputs{
		RandUTXOOutput(iotago.OutputExtended),
		RandUTXOOutput(iotago.OutputFoundry),
		RandUTXOOutput(iotago.OutputExtended), // spent
		RandUTXOOutput(iotago.OutputAlias),
	}
	msIndex := milestone.Index(49)
	msTimestamp := rand.Uint64()
	spents := Spents{
		RandUTXOSpent(previousOutputs[2], msIndex, msTimestamp),
		RandUTXOSpent(outputs[2], msIndex, msTimestamp),
	}
	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	ledgerIndex, err = utxo.ReadLedgerIndex()
	require.NoError(t, err)
	require.Equal(t, msIndex, ledgerIndex)

	// Prepare values to check
	outputByOutputID := make(map[string]struct{})
	unspentByOutputID := make(map[string]struct{})
	for _, output := range previousOutputs {
		outputByOutputID[output.mapKey()] = struct{}{}
		unspentByOutputID[output.mapKey()] = struct{}{}
	}
	for _, output := range outputs {
		outputByOutputID[output.mapKey()] = struct{}{}
		unspentByOutputID[output.mapKey()] = struct{}{}
	}

	spentByOutputID := make(map[string]struct{})
	for _, spent := range previousSpents {
		spentByOutputID[spent.mapKey()] = struct{}{}
		delete(unspentByOutputID, spent.mapKey())
	}
	for _, spent := range spents {
		spentByOutputID[spent.mapKey()] = struct{}{}
		delete(unspentByOutputID, spent.mapKey())
	}

	var outputCount int
	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		outputCount++
		_, has := outputByOutputID[output.mapKey()]
		require.True(t, has)
		delete(outputByOutputID, output.mapKey())
		return true
	}))
	require.Empty(t, outputByOutputID)
	require.Equal(t, 7, outputCount)

	var unspentCount int
	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		unspentCount++
		_, has := unspentByOutputID[output.mapKey()]
		require.True(t, has)
		delete(unspentByOutputID, output.mapKey())
		return true
	}))
	require.Equal(t, 4, unspentCount)
	require.Empty(t, unspentByOutputID)

	var spentCount int
	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		spentCount++
		_, has := spentByOutputID[spent.mapKey()]
		require.True(t, has)
		delete(spentByOutputID, spent.mapKey())
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
		outputByOutputID[output.mapKey()] = struct{}{}
		unspentByOutputID[output.mapKey()] = struct{}{}
	}

	for _, spent := range previousSpents {
		spentByOutputID[spent.mapKey()] = struct{}{}
		delete(unspentByOutputID, spent.mapKey())
	}

	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		_, has := outputByOutputID[output.mapKey()]
		require.True(t, has)
		delete(outputByOutputID, output.mapKey())
		return true
	}))
	require.Empty(t, outputByOutputID)

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		_, has := unspentByOutputID[output.mapKey()]
		require.True(t, has)
		delete(unspentByOutputID, output.mapKey())
		return true
	}))
	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByOutputID[spent.mapKey()]
		require.True(t, has)
		delete(spentByOutputID, spent.mapKey())
		return true
	}))
	require.Empty(t, spentByOutputID)
}
