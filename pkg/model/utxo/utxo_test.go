package utxo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore/mapdb"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

func TestUTXOIterationWithoutFilters(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
	}

	spents := Spents{
		randomSpent(outputs[3]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents))

	// Prepare values to check
	outputByOutputID := make(map[string]struct{})
	unspentByOutputID := make(map[string]struct{})
	for _, output := range outputs {
		outputByOutputID[string(output.OutputID()[:])] = struct{}{}
		unspentByOutputID[string(output.OutputID()[:])] = struct{}{}
	}

	spentByOutputID := make(map[string]struct{})
	for _, spent := range spents {
		spentByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentByOutputID, string(spent.OutputID()[:]))
	}

	// Test iteration without filters
	require.NoError(t, utxo.ForEachOutputWithoutLocking(func(output *Output) bool {
		_, has := outputByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(outputByOutputID, string(output.OutputID()[:]))
		return true
	}))

	require.Empty(t, outputByOutputID)

	require.NoError(t, utxo.ForEachUnspentOutputWithoutLocking(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}, nil))

	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, nil))

	require.Empty(t, spentByOutputID)

}

func TestUTXOIterationWithAddressFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := randomAddress()

	outputs := Outputs{
		randomOutput(iotago.OutputSigLockedSingleOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
	}

	spents := Spents{
		randomSpent(outputs[3]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents))

	// Prepare values to check
	unspentByOutputID := make(map[string]struct{})
	spentByOutputID := make(map[string]struct{})

	// Test iteration with address filter
	unspentByOutputID[string(outputs[0].OutputID()[:])] = struct{}{}
	spentByOutputID[string(outputs[2].OutputID()[:])] = struct{}{}

	require.NoError(t, utxo.ForEachUnspentOutputWithoutLocking(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}, address))

	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, address))

	require.Empty(t, spentByOutputID)
}

func TestUTXOIterationWithAddressAndTypeFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := randomAddress()

	outputs := Outputs{
		randomOutput(iotago.OutputSigLockedSingleOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput, address),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput, address),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput, address),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
	}

	spents := Spents{
		randomSpent(outputs[2]),
		randomSpent(outputs[3]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents))

	// Prepare values to check
	unspentByOutputID := make(map[string]struct{})
	unspentDustByOutputID := make(map[string]struct{})
	spentByOutputID := make(map[string]struct{})
	spentDustByOutputID := make(map[string]struct{})

	// Test iteration with address and type filter
	unspentByOutputID[string(outputs[0].OutputID()[:])] = struct{}{}
	unspentDustByOutputID[string(outputs[4].OutputID()[:])] = struct{}{}
	unspentDustByOutputID[string(outputs[6].OutputID()[:])] = struct{}{}
	unspentDustByOutputID[string(outputs[8].OutputID()[:])] = struct{}{}
	unspentDustByOutputID[string(outputs[9].OutputID()[:])] = struct{}{}

	spentByOutputID[string(outputs[3].OutputID()[:])] = struct{}{}
	spentDustByOutputID[string(outputs[2].OutputID()[:])] = struct{}{}

	require.NoError(t, utxo.ForEachUnspentOutputWithoutLocking(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedSingleOutput))

	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedSingleOutput))

	require.Empty(t, spentByOutputID)

	require.NoError(t, utxo.ForEachUnspentOutputWithoutLocking(func(output *Output) bool {
		_, has := unspentDustByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentDustByOutputID, string(output.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedDustAllowanceOutput))

	require.Empty(t, unspentDustByOutputID)

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentDustByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentDustByOutputID, string(spent.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedDustAllowanceOutput))

	require.Empty(t, spentDustByOutputID)
}

func TestConfirmationApplyAndRollbackToEmptyLedger(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
	}

	spents := Spents{
		randomSpent(outputs[3]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents))

	var outputCount int
	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		outputCount++
		return true
	}))
	require.Equal(t, 5, outputCount)

	var unspentCount int
	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		unspentCount++
		return true
	}, nil))
	require.Equal(t, 3, unspentCount)

	var spentCount int
	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		spentCount++
		return true
	}, nil))
	require.Equal(t, 2, spentCount)

	require.NoError(t, utxo.RollbackConfirmationWithoutLocking(msIndex, outputs, spents))

	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		require.Fail(t, "should not be called")
		return true
	}))

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		require.Fail(t, "should not be called")
		return true
	}, nil))

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		require.Fail(t, "should not be called")
		return true
	}, nil))
}

func TestConfirmationApplyAndRollbackToPreviousLedger(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	previousOutputs := Outputs{
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedDustAllowanceOutput),
	}

	previousSpents := Spents{
		randomSpent(previousOutputs[1]),
	}

	previousMsIndex := milestone.Index(48)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(previousMsIndex, previousOutputs, previousSpents))

	ledgerIndex, err := utxo.ReadLedgerIndex()
	require.NoError(t, err)
	require.Equal(t, previousMsIndex, ledgerIndex)

	outputs := Outputs{
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
		randomOutput(iotago.OutputSigLockedSingleOutput),
	}

	spents := Spents{
		randomSpent(previousOutputs[2]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(49)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents))

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
	}, nil))
	require.Empty(t, unspentByOutputID)
	require.Equal(t, 4, unspentCount)

	var spentCount int
	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		spentCount++
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, nil))
	require.Empty(t, spentByOutputID)
	require.Equal(t, 3, spentCount)

	require.NoError(t, utxo.RollbackConfirmationWithoutLocking(msIndex, outputs, spents))

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
	}, nil))
	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, nil))
	require.Empty(t, spentByOutputID)
}
