package utxo

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestSimpleMilestoneDiffSerialization(t *testing.T) {
	confirmationIndex := milestone.Index(255975)

	outputID := testsuite.RandOutputID()
	messageID := RandMessageID()
	address := testsuite.RandAddress(iotago.AddressEd25519)
	amount := uint64(832493)
	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
	}
	output := CreateOutput(outputID, messageID, confirmationIndex, iotaOutput)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], RandBytes(iotago.TransactionIDLength))

	spent := NewSpent(output, transactionID, confirmationIndex)

	diff := &MilestoneDiff{
		Index:   confirmationIndex,
		Outputs: Outputs{output},
		Spents:  Spents{spent},
	}

	confirmationIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(confirmationIndexBytes, uint32(confirmationIndex))

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, confirmationIndexBytes), diff.kvStorableKey())

	value := diff.kvStorableValue()
	require.Equal(t, len(value), 77)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(0))
}

func TestTreasuryMilestoneDiffSerialization(t *testing.T) {
	outputID := testsuite.RandOutputID()
	messageID := RandMessageID()
	address := testsuite.RandAddress(iotago.AddressEd25519)
	amount := uint64(235234)
	msIndex := testsuite.RandMilestoneIndex()
	iotaOutput := &iotago.ExtendedOutput{
		Address: address,
		Amount:  amount,
	}
	output := CreateOutput(outputID, messageID, msIndex, iotaOutput)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], RandBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(255975)

	spent := NewSpent(output, transactionID, confirmationIndex)

	spentMilestoneID := iotago.MilestoneID{}
	copy(spentMilestoneID[:], RandBytes(iotago.MilestoneIDLength))

	spentTreasuryOutput := &TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], RandBytes(iotago.MilestoneIDLength))

	treasuryOutput := &TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	diff := &MilestoneDiff{
		Index:               confirmationIndex,
		Outputs:             Outputs{output},
		Spents:              Spents{spent},
		SpentTreasuryOutput: spentTreasuryOutput,
		TreasuryOutput:      treasuryOutput,
	}

	confirmationIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(confirmationIndexBytes, uint32(confirmationIndex))

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, confirmationIndexBytes), diff.kvStorableKey())

	value := diff.kvStorableValue()
	require.Equal(t, len(value), 141)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(1))
	require.Equal(t, value[77:109], milestoneID[:])
	require.Equal(t, value[109:141], spentMilestoneID[:])
}

func EqualOutputs(t *testing.T, expected Outputs, actual Outputs) {
	require.Equal(t, len(expected), len(actual))

	for i := 0; i < len(expected); i++ {
		EqualOutput(t, expected[i], actual[i])
	}
}

func EqualSpents(t *testing.T, expected Spents, actual Spents) {
	require.Equal(t, len(expected), len(actual))

	for i := 0; i < len(expected); i++ {
		EqualSpent(t, expected[i], actual[i])
	}
}

func TestMilestoneDiffSerialization(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		testsuite.RandOutput(iotago.OutputExtended),
		testsuite.RandOutput(iotago.OutputExtended),
		testsuite.RandOutput(iotago.OutputExtended),
		testsuite.RandOutput(iotago.OutputExtended),
		testsuite.RandOutput(iotago.OutputExtended),
	}

	msIndex := milestone.Index(756)

	spents := Spents{
		testsuite.RandSpent(outputs[3], msIndex),
		testsuite.RandSpent(outputs[2], msIndex),
	}

	spentMilestoneID := iotago.MilestoneID{}
	copy(spentMilestoneID[:], RandBytes(iotago.MilestoneIDLength))

	spentTreasuryOutput := &TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], RandBytes(iotago.MilestoneIDLength))

	treasuryOutput := &TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	treasuryTuple := &TreasuryMutationTuple{
		NewOutput:   treasuryOutput,
		SpentOutput: spentTreasuryOutput,
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, treasuryTuple, nil))

	readDiff, err := utxo.MilestoneDiffWithoutLocking(msIndex)
	require.NoError(t, err)

	require.Equal(t, msIndex, readDiff.Index)
	EqualOutputs(t, outputs, readDiff.Outputs)
	EqualSpents(t, spents, readDiff.Spents)
	require.Equal(t, treasuryOutput, readDiff.TreasuryOutput)
	require.Equal(t, spentTreasuryOutput, readDiff.SpentTreasuryOutput)
}
