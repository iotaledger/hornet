//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package utxo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestUTXOComputeBalance(t *testing.T) {

	manager := utxo.New(mapdb.NewMapDB())

	initialOutput := tpkg.RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressEd25519), 2_134_656_365)
	require.NoError(t, manager.AddUnspentOutput(initialOutput))
	require.NoError(t, manager.AddUnspentOutput(tpkg.RandUTXOOutputOnAddressWithAmount(iotago.OutputAlias, tpkg.RandAddress(iotago.AddressAlias), 56_549_524)))
	require.NoError(t, manager.AddUnspentOutput(tpkg.RandUTXOOutputOnAddressWithAmount(iotago.OutputFoundry, tpkg.RandAddress(iotago.AddressAlias), 25_548_858)))
	require.NoError(t, manager.AddUnspentOutput(tpkg.RandUTXOOutputOnAddressWithAmount(iotago.OutputNFT, tpkg.RandAddress(iotago.AddressEd25519), 545_699_656)))
	require.NoError(t, manager.AddUnspentOutput(tpkg.RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressAlias), 626_659_696)))

	msIndex := iotago.MilestoneIndex(756)
	msTimestamp := tpkg.RandMilestoneTimestamp()

	outputs := utxo.Outputs{
		tpkg.RandUTXOOutputOnAddressWithAmount(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressNFT), 2_134_656_365),
	}

	spents := utxo.Spents{
		tpkg.RandUTXOSpentWithOutput(initialOutput, msIndex, msTimestamp),
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
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressEd25519)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressNFT)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressAlias)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressEd25519)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressNFT)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressAlias)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputBasic, tpkg.RandAddress(iotago.AddressEd25519)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputNFT, tpkg.RandAddress(iotago.AddressEd25519)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputNFT, tpkg.RandAddress(iotago.AddressAlias)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputNFT, tpkg.RandAddress(iotago.AddressNFT)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputNFT, tpkg.RandAddress(iotago.AddressAlias)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputAlias, tpkg.RandAddress(iotago.AddressEd25519)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputFoundry, tpkg.RandAddress(iotago.AddressAlias)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputFoundry, tpkg.RandAddress(iotago.AddressAlias)),
		tpkg.RandUTXOOutputOnAddress(iotago.OutputFoundry, tpkg.RandAddress(iotago.AddressAlias)),
	}

	msIndex := iotago.MilestoneIndex(756)
	msTimestamp := tpkg.RandMilestoneTimestamp()

	spents := utxo.Spents{
		tpkg.RandUTXOSpentWithOutput(outputs[3], msIndex, msTimestamp),
		tpkg.RandUTXOSpentWithOutput(outputs[2], msIndex, msTimestamp),
		tpkg.RandUTXOSpentWithOutput(outputs[9], msIndex, msTimestamp),
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
