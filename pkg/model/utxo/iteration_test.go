package utxo

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/utxo/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestUTXOComputeBalance(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	initialOutput := RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, utils.RandAddress(iotago.AddressEd25519), 2_134_656_365)
	require.NoError(t, utxo.AddUnspentOutput(initialOutput))
	require.NoError(t, utxo.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputAlias, utils.RandAddress(iotago.AddressAlias), 56_549_524)))
	require.NoError(t, utxo.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputFoundry, utils.RandAddress(iotago.AddressAlias), 25_548_858)))
	require.NoError(t, utxo.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputNFT, utils.RandAddress(iotago.AddressEd25519), 545_699_656)))
	require.NoError(t, utxo.AddUnspentOutput(RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, utils.RandAddress(iotago.AddressAlias), 626_659_696)))

	msIndex := milestone.Index(756)
	msTimestamp := rand.Uint32()

	outputs := Outputs{
		RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, utils.RandAddress(iotago.AddressNFT), 2_134_656_365),
	}

	spents := Spents{
		RandUTXOSpent(initialOutput, msIndex, msTimestamp),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	spent, err := utxo.SpentOutputs()
	require.NoError(t, err)
	require.Equal(t, 1, len(spent))

	unspent, err := utxo.UnspentOutputs()
	require.NoError(t, err)
	require.Equal(t, 5, len(unspent))

	balance, count, err := utxo.ComputeLedgerBalance()
	require.NoError(t, err)
	require.Equal(t, 5, count)
	require.Equal(t, uint64(2_134_656_365+56_549_524+25_548_858+545_699_656+626_659_696), balance)
}

func TestUTXOIteration(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
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

	spents := Spents{
		RandUTXOSpent(outputs[3], msIndex, msTimestamp),
		RandUTXOSpent(outputs[2], msIndex, msTimestamp),
		RandUTXOSpent(outputs[9], msIndex, msTimestamp),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Prepare values to check
	outputByID := make(map[string]struct{})
	unspentByID := make(map[string]struct{})
	spentByID := make(map[string]struct{})

	for _, output := range outputs {
		outputByID[output.mapKey()] = struct{}{}
		unspentByID[output.mapKey()] = struct{}{}
	}
	for _, spent := range spents {
		spentByID[spent.mapKey()] = struct{}{}
		delete(unspentByID, spent.mapKey())
	}

	// Test iteration without filters
	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		_, has := outputByID[output.mapKey()]
		require.True(t, has)
		delete(outputByID, output.mapKey())
		return true
	}))

	require.Empty(t, outputByID)

	require.NoError(t, utxo.ForEachUnspentOutput(func(output *Output) bool {
		_, has := unspentByID[output.mapKey()]
		require.True(t, has)
		delete(unspentByID, output.mapKey())
		return true
	}))
	require.Empty(t, unspentByID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByID[spent.mapKey()]
		require.True(t, has)
		delete(spentByID, spent.mapKey())
		return true
	}))

	require.Empty(t, spentByID)
}
