package utxo_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/model/utxo/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestUTXOComputeBalance(t *testing.T) {

	manager := utxo.New(mapdb.NewMapDB())

	initialOutput := RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, utils.RandAddress(iotago.AddressEd25519), 2_134_656_365)
	require.NoError(t, manager.AddUnspentOutput(initialOutput))
	require.NoError(t, manager.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputAlias, utils.RandAddress(iotago.AddressAlias), 56_549_524)))
	require.NoError(t, manager.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputFoundry, utils.RandAddress(iotago.AddressAlias), 25_548_858)))
	require.NoError(t, manager.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputNFT, utils.RandAddress(iotago.AddressEd25519), 545_699_656)))
	require.NoError(t, manager.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, utils.RandAddress(iotago.AddressAlias), 626_659_696)))

	msIndex := milestone.Index(756)
	msTimestamp := rand.Uint32()

	outputs := utxo.Outputs{
		RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, utils.RandAddress(iotago.AddressNFT), 2_134_656_365),
	}

	spents := utxo.Spents{
		RandUTXOSpent(initialOutput, msIndex, msTimestamp),
	}

	require.NoError(t, manager.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	spent, err := manager.SpentOutputs()
	require.NoError(t, err)
	require.Equal(t, 1, len(spent))

	unspent, err := manager.UnspentOutputs()
	require.NoError(t, err)
	require.Equal(t, 5, len(unspent))

	balance, count, err := manager.ComputeLedgerBalance()
	require.NoError(t, err)
	require.Equal(t, 5, count)
	require.Equal(t, uint64(2_134_656_365+56_549_524+25_548_858+545_699_656+626_659_696), balance)
}

func TestUTXOIteration(t *testing.T) {

	manager := utxo.New(mapdb.NewMapDB())

	outputs := utxo.Outputs{
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressEd25519)),
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressNFT)),
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressAlias)),
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressEd25519)),
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressNFT)),
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressAlias)),
		RandUTXOOutputOnAddress(iotago.OutputBasic, utils.RandAddress(iotago.AddressEd25519)),
		RandUTXOOutputOnAddress(iotago.OutputNFT, utils.RandAddress(iotago.AddressEd25519)),
		RandUTXOOutputOnAddress(iotago.OutputNFT, utils.RandAddress(iotago.AddressAlias)),
		RandUTXOOutputOnAddress(iotago.OutputNFT, utils.RandAddress(iotago.AddressNFT)),
		RandUTXOOutputOnAddress(iotago.OutputNFT, utils.RandAddress(iotago.AddressAlias)),
		RandUTXOOutputOnAddress(iotago.OutputAlias, utils.RandAddress(iotago.AddressEd25519)),
		RandUTXOOutputOnAddress(iotago.OutputFoundry, utils.RandAddress(iotago.AddressAlias)),
		RandUTXOOutputOnAddress(iotago.OutputFoundry, utils.RandAddress(iotago.AddressAlias)),
		RandUTXOOutputOnAddress(iotago.OutputFoundry, utils.RandAddress(iotago.AddressAlias)),
	}

	msIndex := milestone.Index(756)
	msTimestamp := rand.Uint32()

	spents := utxo.Spents{
		RandUTXOSpent(outputs[3], msIndex, msTimestamp),
		RandUTXOSpent(outputs[2], msIndex, msTimestamp),
		RandUTXOSpent(outputs[9], msIndex, msTimestamp),
	}

	require.NoError(t, manager.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Prepare values to check
	outputByID := make(map[string]struct{})
	unspentByID := make(map[string]struct{})
	spentByID := make(map[string]struct{})

	for _, output := range outputs {
		outputByID[output.MapKey()] = struct{}{}
		unspentByID[output.MapKey()] = struct{}{}
	}
	for _, spent := range spents {
		spentByID[spent.MapKey()] = struct{}{}
		delete(unspentByID, spent.MapKey())
	}

	// Test iteration without filters
	require.NoError(t, manager.ForEachOutput(func(output *utxo.Output) bool {
		_, has := outputByID[output.MapKey()]
		require.True(t, has)
		delete(outputByID, output.MapKey())
		return true
	}))

	require.Empty(t, outputByID)

	require.NoError(t, manager.ForEachUnspentOutput(func(output *utxo.Output) bool {
		_, has := unspentByID[output.MapKey()]
		require.True(t, has)
		delete(unspentByID, output.MapKey())
		return true
	}))
	require.Empty(t, unspentByID)

	require.NoError(t, manager.ForEachSpentOutput(func(spent *utxo.Spent) bool {
		_, has := spentByID[spent.MapKey()]
		require.True(t, has)
		delete(spentByID, spent.MapKey())
		return true
	}))

	require.Empty(t, spentByID)
}
