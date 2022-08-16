//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package utxo_test

import (
	"encoding/binary"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestSimpleMilestoneDiffSerialization(t *testing.T) {
	msIndexBooked := iotago.MilestoneIndex(255975)
	msTimestampBooked := tpkg.RandMilestoneTimestamp()

	outputID := tpkg.RandOutputID()
	blockID := tpkg.RandBlockID()
	address := tpkg.RandAddress(iotago.AddressEd25519)
	amount := uint64(832493)
	iotaOutput := &iotago.BasicOutput{
		Amount: amount,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
		},
	}
	output := utxo.CreateOutput(outputID, blockID, msIndexBooked, msTimestampBooked, iotaOutput)

	transactionIDSpent := tpkg.RandTransactionID()

	msIndexSpent := msIndexBooked + 1
	msTimestampSpent := msTimestampBooked + 1

	spent := utxo.NewSpent(output, transactionIDSpent, msIndexSpent, msTimestampSpent)

	diff := &utxo.MilestoneDiff{
		Index:   msIndexSpent,
		Outputs: utxo.Outputs{output},
		Spents:  utxo.Spents{spent},
	}

	milestoneIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(milestoneIndexBytes, msIndexSpent)

	require.Equal(t, byteutils.ConcatBytes([]byte{utxo.UTXOStoreKeyPrefixMilestoneDiffs}, milestoneIndexBytes), diff.KVStorableKey())

	value := diff.KVStorableValue()
	require.Equal(t, len(value), 77)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(0))
}

func TestTreasuryMilestoneDiffSerialization(t *testing.T) {
	outputID := tpkg.RandOutputID()
	blockID := tpkg.RandBlockID()
	address := tpkg.RandAddress(iotago.AddressEd25519)
	amount := uint64(235234)
	msIndexBooked := tpkg.RandMilestoneIndex()
	msTimestampBooked := tpkg.RandMilestoneTimestamp()
	iotaOutput := &iotago.BasicOutput{
		Amount: amount,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
		},
	}
	output := utxo.CreateOutput(outputID, blockID, msIndexBooked, msTimestampBooked, iotaOutput)

	transactionIDSpent := tpkg.RandTransactionID()

	msIndexSpent := iotago.MilestoneIndex(255975)
	msTimestampSpent := tpkg.RandMilestoneTimestamp()

	spent := utxo.NewSpent(output, transactionIDSpent, msIndexSpent, msTimestampSpent)

	spentMilestoneID := tpkg.RandMilestoneID()
	spentTreasuryOutput := &utxo.TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := tpkg.RandMilestoneID()
	treasuryOutput := &utxo.TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	diff := &utxo.MilestoneDiff{
		Index:               msIndexSpent,
		Outputs:             utxo.Outputs{output},
		Spents:              utxo.Spents{spent},
		SpentTreasuryOutput: spentTreasuryOutput,
		TreasuryOutput:      treasuryOutput,
	}

	milestoneIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(milestoneIndexBytes, msIndexSpent)

	require.Equal(t, byteutils.ConcatBytes([]byte{utxo.UTXOStoreKeyPrefixMilestoneDiffs}, milestoneIndexBytes), diff.KVStorableKey())

	value := diff.KVStorableValue()
	require.Equal(t, len(value), 141)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(1))
	require.Equal(t, value[77:109], milestoneID[:])
	require.Equal(t, value[109:141], spentMilestoneID[:])
}

func TestMilestoneDiffSerialization(t *testing.T) {

	manager := utxo.New(mapdb.NewMapDB())

	outputs := utxo.Outputs{
		tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
		tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
		tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
		tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
		tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
	}

	msIndex := iotago.MilestoneIndex(756)
	msTimestamp := tpkg.RandMilestoneTimestamp()

	spents := utxo.Spents{
		tpkg.RandUTXOSpentWithOutput(outputs[3], msIndex, msTimestamp),
		tpkg.RandUTXOSpentWithOutput(outputs[2], msIndex, msTimestamp),
	}

	spentMilestoneID := tpkg.RandMilestoneID()

	spentTreasuryOutput := &utxo.TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := tpkg.RandMilestoneID()

	treasuryOutput := &utxo.TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	treasuryTuple := &utxo.TreasuryMutationTuple{
		NewOutput:   treasuryOutput,
		SpentOutput: spentTreasuryOutput,
	}

	require.NoError(t, manager.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, treasuryTuple, nil))

	readDiff, err := manager.MilestoneDiffWithoutLocking(msIndex)
	require.NoError(t, err)

	var sortedOutputs = utxo.LexicalOrderedOutputs(outputs)
	sort.Sort(sortedOutputs)

	var sortedSpents = utxo.LexicalOrderedSpents(spents)
	sort.Sort(sortedSpents)

	require.Equal(t, msIndex, readDiff.Index)
	tpkg.EqualOutputs(t, utxo.Outputs(sortedOutputs), readDiff.Outputs)
	tpkg.EqualSpents(t, utxo.Spents(sortedSpents), readDiff.Spents)
	require.Equal(t, treasuryOutput, readDiff.TreasuryOutput)
	require.Equal(t, spentTreasuryOutput, readDiff.SpentTreasuryOutput)
}
