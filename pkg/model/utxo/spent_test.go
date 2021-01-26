package utxo

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

func EqualSpents(t *testing.T, expected *Spent, actual *Spent) {
	require.True(t, bytes.Equal(expected.outputID[:], actual.outputID[:]))
	require.True(t, bytes.Equal(expected.TargetTransactionID()[:], actual.TargetTransactionID()[:]))
	require.Equal(t, expected.ConfirmationIndex(), actual.ConfirmationIndex())
	EqualOutputs(t, expected.Output(), actual.Output())
}

func TestSpentSerialization(t *testing.T) {

	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], randBytes(iotago.TransactionIDLength+iotago.UInt16ByteSize))

	messageID := &hornet.MessageID{}
	copy(messageID[:], randBytes(iotago.MessageIDLength))

	outputType := iotago.OutputSigLockedDustAllowanceOutput

	address := &iotago.Ed25519Address{}
	addressBytes := randBytes(32)
	copy(address[:], addressBytes)

	amount := uint64(832493)

	output := CreateOutput(outputID, messageID, outputType, address, amount)

	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(6788362)

	spent := NewSpent(output, transactionID, confirmationIndex)

	require.True(t, bytes.Equal(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, []byte{iotago.AddressEd25519}, addressBytes, []byte{outputType}, outputID[:]), spent.kvStorableKey()))

	value := spent.kvStorableValue()
	require.True(t, bytes.Equal(transactionID[:], value[:32]))
	require.Equal(t, confirmationIndex, milestone.Index(binary.LittleEndian.Uint32(value[32:36])))

	store := mapdb.NewMapDB()

	utxo := New(store)

	require.NoError(t, utxo.ApplyConfirmation(confirmationIndex, Outputs{output}, Spents{spent}))

	readSpent, err := utxo.readSpentForOutputIDWithoutLocking(outputID)
	require.NoError(t, err)

	EqualSpents(t, spent, readSpent)

}
