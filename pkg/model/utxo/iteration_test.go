package utxo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestUTXOIterationWithoutFilters(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
	}

	msIndex := milestone.Index(756)

	spents := Spents{
		randomSpent(outputs[3], msIndex),
		randomSpent(outputs[2], msIndex),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Prepare values to check
	outputByOutputID := make(map[string]struct{})
	unspentByOutputID := make(map[string]struct{})
	spentByOutputID := make(map[string]struct{})

	for _, output := range outputs {
		outputByOutputID[string(output.OutputID()[:])] = struct{}{}
		unspentByOutputID[string(output.OutputID()[:])] = struct{}{}
	}

	for _, spent := range spents {
		spentByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentByOutputID, string(spent.OutputID()[:]))
	}

	// Test iteration without filters
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

func TestUTXOIterationWithAddressFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := randomAddress()

	outputs := Outputs{
		randomOutput(iotago.OutputExtended, address),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended, address),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
	}

	spents := Spents{
		randomSpent(outputs[3]),
		randomSpent(outputs[2]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Prepare values to check
	unspentByOutputID := make(map[string]struct{})
	spentByOutputID := make(map[string]struct{})

	// Test iteration with address filter
	unspentByOutputID[string(outputs[0].OutputID()[:])] = struct{}{}
	spentByOutputID[string(outputs[2].OutputID()[:])] = struct{}{}

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}, FilterAddress(address)))

	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, FilterAddress(address)))

	require.Empty(t, spentByOutputID)
}

func TestUTXOIterationWithAddressAndTypeFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := randomAddress()

	outputs := Outputs{
		randomOutput(iotago.OutputExtended, address),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputNFT, address),
		randomOutput(iotago.OutputExtended, address),
		randomOutput(iotago.OutputNFT, address),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputNFT, address),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputNFT, address),
		randomOutput(iotago.OutputNFT, address),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
	}

	spents := Spents{
		randomSpent(outputs[2]),
		randomSpent(outputs[3]),
	}

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Prepare values to check
	unspentByOutputID := make(map[string]struct{})
	unspentNFTByOutputID := make(map[string]struct{})
	spentByOutputID := make(map[string]struct{})
	spentNFTByOutputID := make(map[string]struct{})

	// Test iteration with address and type filter
	unspentByOutputID[string(outputs[0].OutputID()[:])] = struct{}{}
	unspentNFTByOutputID[string(outputs[4].OutputID()[:])] = struct{}{}
	unspentNFTByOutputID[string(outputs[6].OutputID()[:])] = struct{}{}
	unspentNFTByOutputID[string(outputs[8].OutputID()[:])] = struct{}{}
	unspentNFTByOutputID[string(outputs[9].OutputID()[:])] = struct{}{}

	spentByOutputID[string(outputs[3].OutputID()[:])] = struct{}{}
	spentNFTByOutputID[string(outputs[2].OutputID()[:])] = struct{}{}

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		_, has := unspentByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentByOutputID, string(output.OutputID()[:]))
		return true
	}, FilterAddress(address), FilterOutputType(iotago.OutputExtended)))

	require.Empty(t, unspentByOutputID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentByOutputID, string(spent.OutputID()[:]))
		return true
	}, FilterAddress(address), FilterOutputType(iotago.OutputNFT)))

	require.Empty(t, spentByOutputID)

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		_, has := unspentNFTByOutputID[string(output.OutputID()[:])]
		require.True(t, has)
		delete(unspentNFTByOutputID, string(output.OutputID()[:]))
		return true
	}, FilterAddress(address), FilterOutputType(iotago.OutputNFT)))

	require.Empty(t, unspentNFTByOutputID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentNFTByOutputID[string(spent.OutputID()[:])]
		require.True(t, has)
		delete(spentNFTByOutputID, string(spent.OutputID()[:]))
		return true
	}, FilterAddress(address), FilterOutputType(iotago.OutputNFT)))

	require.Empty(t, spentNFTByOutputID)
}

func TestUTXOLoadMethodsWithIterateOptions(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := randomAddress()

	outputs := Outputs{
		randomOutputOnAddressWithAmount(iotago.OutputExtended, address, 5_000_000),
		randomOutput(iotago.OutputExtended),
		randomOutputOnAddressWithAmount(iotago.OutputExtended, address, 150),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutputOnAddressWithAmount(iotago.OutputExtended, address, 3_125_125),
		randomOutput(iotago.OutputExtended),
		randomOutput(iotago.OutputExtended),
		randomOutputOnAddressWithAmount(iotago.OutputExtended, address, 89_923_223),
		randomOutput(iotago.OutputExtended),
	}
	nftOutputs := Outputs{
		randomOutputOnAddressWithAmount(iotago.OutputNFT, address, 1_000_000),
		randomOutput(iotago.OutputNFT),
		randomOutputOnAddressWithAmount(iotago.OutputNFT, address, 5_500_000),
		randomOutput(iotago.OutputNFT),
		randomOutputOnAddressWithAmount(iotago.OutputNFT, address, 1_000_000),
	}

	spents := Spents{
		randomSpent(outputs[2]),
		randomSpent(outputs[3]),
	}
	nftSpents := Spents{
		randomSpent(nftOutputs[2]),
		randomSpent(nftOutputs[4]),
	}

	expectedBalanceOnAddress := uint64(105_548_498 - 6_500_150)

	msIndex := milestone.Index(756)

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, append(outputs, nftOutputs...), append(spents, nftSpents...), nil, nil))

	// Prepare values to check
	unspentByOutputID := make(map[string]struct{})
	unspentNFTByOutputID := make(map[string]struct{})
	spentByOutputID := make(map[string]struct{})
	spentNFTByOutputID := make(map[string]struct{})

	for _, output := range outputs {
		unspentByOutputID[string(output.OutputID()[:])] = struct{}{}
	}

	for _, output := range nftOutputs {
		unspentNFTByOutputID[string(output.OutputID()[:])] = struct{}{}
	}

	for _, spent := range spents {
		spentByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentByOutputID, string(spent.OutputID()[:]))
	}

	for _, spent := range nftSpents {
		spentNFTByOutputID[string(spent.OutputID()[:])] = struct{}{}
		delete(unspentNFTByOutputID, string(spent.OutputID()[:]))
	}

	// Test no MaxResultCount
	loadedSpents, err := utxo.SpentOutputs(FilterAddress(address))
	require.NoError(t, err)
	require.Equal(t, 3, len(loadedSpents))

	loadedUnspent, err := utxo.UnspentOutputs(FilterAddress(address))
	require.NoError(t, err)
	require.Equal(t, 4, len(loadedUnspent))

	computedBalance, count, err := utxo.ComputeBalance(FilterAddress(address))
	require.NoError(t, err)
	require.Equal(t, 4, count)
	require.Equal(t, expectedBalanceOnAddress, computedBalance)

	// Test MaxResultCount
	loadedSpents, err = utxo.SpentOutputs(FilterAddress(address), MaxResultCount(2))
	require.NoError(t, err)
	require.Equal(t, 2, len(loadedSpents))

	loadedUnspent, err = utxo.UnspentOutputs(FilterAddress(address), MaxResultCount(2))
	require.NoError(t, err)
	require.Equal(t, 2, len(loadedUnspent))

	computedBalance, count, err = utxo.ComputeBalance(FilterAddress(address), MaxResultCount(2))
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.NotEqual(t, expectedBalanceOnAddress, computedBalance)

	// Test OutputType = Extended Output
	loadedSpents, err = utxo.SpentOutputs(FilterAddress(address), FilterOutputType(iotago.OutputExtended))
	require.NoError(t, err)
	require.Equal(t, 1, len(loadedSpents))

	loadedUnspent, err = utxo.UnspentOutputs(FilterAddress(address), FilterOutputType(iotago.OutputExtended))
	require.NoError(t, err)
	require.Equal(t, 3, len(loadedUnspent))

	// Test OutputType = NFT Output
	loadedSpents, err = utxo.SpentOutputs(FilterAddress(address), FilterOutputType(iotago.OutputNFT))
	require.NoError(t, err)
	require.Equal(t, 2, len(loadedSpents))

	loadedUnspent, err = utxo.UnspentOutputs(FilterAddress(address), FilterOutputType(iotago.OutputNFT))
	require.NoError(t, err)
	require.Equal(t, 1, len(loadedUnspent))
}
