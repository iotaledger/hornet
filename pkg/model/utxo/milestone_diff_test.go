package utxo

import (
	"encoding/binary"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo/utils"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestSimpleMilestoneDiffSerialization(t *testing.T) {
	milestoneIndex := milestone.Index(255975)
	milestoneTimestamp := rand.Uint64()

	outputID := utils.RandOutputID()
	messageID := utils.RandMessageID()
	address := utils.RandAddress(iotago.AddressEd25519)
	amount := uint64(832493)
	iotaOutput := &iotago.BasicOutput{
		Amount: amount,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
		},
	}
	output := CreateOutput(outputID, messageID, milestoneIndex, milestoneTimestamp, iotaOutput)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], utils.RandBytes(iotago.TransactionIDLength))

	spent := NewSpent(output, transactionID, milestoneIndex, milestoneTimestamp)

	diff := &MilestoneDiff{
		Index:   milestoneIndex,
		Outputs: Outputs{output},
		Spents:  Spents{spent},
	}

	milestoneIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(milestoneIndexBytes, uint32(milestoneIndex))

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, milestoneIndexBytes), diff.kvStorableKey())

	value := diff.kvStorableValue()
	require.Equal(t, len(value), 77)
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, outputID[:], value[42:76])
	require.Equal(t, value[76], byte(0))
}

func TestTreasuryMilestoneDiffSerialization(t *testing.T) {
	outputID := utils.RandOutputID()
	messageID := utils.RandMessageID()
	address := utils.RandAddress(iotago.AddressEd25519)
	amount := uint64(235234)
	msIndex := utils.RandMilestoneIndex()
	msTimestamp := rand.Uint64()
	iotaOutput := &iotago.BasicOutput{
		Amount: amount,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: address,
			},
		},
	}
	output := CreateOutput(outputID, messageID, msIndex, msTimestamp, iotaOutput)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], utils.RandBytes(iotago.TransactionIDLength))

	milestoneIndex := milestone.Index(255975)
	milestoneTimestamp := rand.Uint64()

	spent := NewSpent(output, transactionID, milestoneIndex, milestoneTimestamp)

	spentMilestoneID := iotago.MilestoneID{}
	copy(spentMilestoneID[:], utils.RandBytes(iotago.MilestoneIDLength))

	spentTreasuryOutput := &TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], utils.RandBytes(iotago.MilestoneIDLength))

	treasuryOutput := &TreasuryOutput{
		MilestoneID: milestoneID,
		Amount:      0,
		Spent:       false,
	}

	diff := &MilestoneDiff{
		Index:               milestoneIndex,
		Outputs:             Outputs{output},
		Spents:              Spents{spent},
		SpentTreasuryOutput: spentTreasuryOutput,
		TreasuryOutput:      treasuryOutput,
	}

	milestoneIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(milestoneIndexBytes, uint32(milestoneIndex))

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixMilestoneDiffs}, milestoneIndexBytes), diff.kvStorableKey())

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

func TestMilestoneDiffSerialization(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	outputs := Outputs{
		RandUTXOOutput(iotago.OutputBasic),
		RandUTXOOutput(iotago.OutputBasic),
		RandUTXOOutput(iotago.OutputBasic),
		RandUTXOOutput(iotago.OutputBasic),
		RandUTXOOutput(iotago.OutputBasic),
	}

	msIndex := milestone.Index(756)
	msTimestamp := rand.Uint64()

	spents := Spents{
		RandUTXOSpent(outputs[3], msIndex, msTimestamp),
		RandUTXOSpent(outputs[2], msIndex, msTimestamp),
	}

	spentMilestoneID := iotago.MilestoneID{}
	copy(spentMilestoneID[:], utils.RandBytes(iotago.MilestoneIDLength))

	spentTreasuryOutput := &TreasuryOutput{
		MilestoneID: spentMilestoneID,
		Amount:      1337,
		Spent:       true,
	}

	milestoneID := iotago.MilestoneID{}
	copy(milestoneID[:], utils.RandBytes(iotago.MilestoneIDLength))

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

	var sortedOutputs LexicalOrderedOutputs = LexicalOrderedOutputs(outputs)
	sort.Sort(sortedOutputs)

	var sortedSpents LexicalOrderedSpents = LexicalOrderedSpents(spents)
	sort.Sort(sortedSpents)

	require.Equal(t, msIndex, readDiff.Index)
	EqualOutputs(t, Outputs(sortedOutputs), readDiff.Outputs)
	EqualSpents(t, Spents(sortedSpents), readDiff.Spents)
	require.Equal(t, treasuryOutput, readDiff.TreasuryOutput)
	require.Equal(t, spentTreasuryOutput, readDiff.SpentTreasuryOutput)
}
