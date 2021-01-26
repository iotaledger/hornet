package utxo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore/mapdb"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

func TestUTXOIterationWithoutFilters(t *testing.T) {

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

	utxo := New(mapdb.NewMapDB())

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

	require.Equal(t, 0, len(outputByOutputID))

	require.NoError(t, utxo.ForEachUnspentOutputWithoutLocking(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}, nil))

	require.Equal(t, 0, len(unspentByOutputID))

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, nil))

	require.Equal(t, 0, len(spentByOutputID))

}

func TestUTXOIterationWithAddressFilter(t *testing.T) {

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

	utxo := New(mapdb.NewMapDB())

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

	require.Equal(t, 0, len(unspentByOutputID))

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, address))

	require.Equal(t, 0, len(spentByOutputID))
}

func TestUTXOIterationWithAddressAndTypeFilter(t *testing.T) {

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

	utxo := New(mapdb.NewMapDB())

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

	require.Equal(t, 0, len(unspentByOutputID))

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedSingleOutput))

	require.Equal(t, 0, len(spentByOutputID))

	require.NoError(t, utxo.ForEachUnspentOutputWithoutLocking(func(output *Output) bool {
		_, has := unspentDustByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentDustByOutputID, string(output.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedDustAllowanceOutput))

	require.Equal(t, 0, len(unspentDustByOutputID))

	require.NoError(t, utxo.ForEachSpentOutputWithoutLocking(func(spent *Spent) bool {
		_, has := spentDustByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentDustByOutputID, string(spent.OutputID()[:]))
		return true
	}, address, iotago.OutputSigLockedDustAllowanceOutput))

	require.Equal(t, 0, len(spentDustByOutputID))
}
