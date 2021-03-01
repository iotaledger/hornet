package utxo

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

func TestSimpleMilestoneDiffSerialization(t *testing.T) {

	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], randBytes(34))

	messageID := randMessageID()

	outputType := iotago.OutputSigLockedDustAllowanceOutput

	address := randomAddress()

	amount := uint64(832493)

	output := CreateOutput(outputID, messageID, outputType, address, amount)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(255975)

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

	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[:4]))
	require.Equal(t, outputID[:], value[4:38])
	require.Equal(t, uint32(1), binary.LittleEndian.Uint32(value[38:42]))
	require.Equal(t, byteutils.ConcatBytes([]byte{iotago.AddressEd25519}, address[:], outputID[:]), value[42:109])
}

func randomAddress() *iotago.Ed25519Address {
	address := &iotago.Ed25519Address{}
	addressBytes := randBytes(32)
	copy(address[:], addressBytes)
	return address
}

func randomOutput(outputType iotago.OutputType, address ...iotago.Address) *Output {
	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], randBytes(34))

	messageID := randMessageID()

	var addr iotago.Address
	if len(address) > 0 {
		addr = address[0]
	} else {
		addr = randomAddress()
	}

	amount := uint64(rand.Intn(2156465))

	return CreateOutput(outputID, messageID, outputType, addr, amount)
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

func randomSpent(output *Output) *Spent {
	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(rand.Intn(216589))

	return NewSpent(output, transactionID, confirmationIndex)
}

func TestMilestoneDiffSerialization(t *testing.T) {

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

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	readDiff, err := utxo.GetMilestoneDiffWithoutLocking(msIndex)
	require.NoError(t, err)

	require.Equal(t, msIndex, readDiff.Index)
	EqualOutputs(t, outputs, readDiff.Outputs)
	EqualSpents(t, spents, readDiff.Spents)
}
