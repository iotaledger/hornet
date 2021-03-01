package utxo

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v2"
)

func EqualSpent(t *testing.T, expected *Spent, actual *Spent) {
	require.Equal(t, expected.outputID[:], actual.outputID[:])
	require.Equal(t, expected.TargetTransactionID()[:], actual.TargetTransactionID()[:])
	require.Equal(t, expected.ConfirmationIndex(), actual.ConfirmationIndex())
	EqualOutput(t, expected.Output(), actual.Output())
}

func TestSpentSerialization(t *testing.T) {

	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], randBytes(iotago.TransactionIDLength+iotago.UInt16ByteSize))

	messageID := randMessageID()

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

	require.Equal(t, byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, []byte{iotago.AddressEd25519}, addressBytes, []byte{outputType}, outputID[:]), spent.kvStorableKey())

	value := spent.kvStorableValue()
	require.Equal(t, transactionID[:], value[:32])
	require.Equal(t, confirmationIndex, milestone.Index(binary.LittleEndian.Uint32(value[32:36])))

	store := mapdb.NewMapDB()

	utxo := New(store)

	require.NoError(t, utxo.ApplyConfirmation(confirmationIndex, Outputs{output}, Spents{spent}, nil, nil))

	readSpent, err := utxo.readSpentForOutputIDWithoutLocking(outputID)
	require.NoError(t, err)

	EqualSpent(t, spent, readSpent)

	unspent, err := utxo.IsOutputUnspent(outputID)
	require.NoError(t, err)
	require.False(t, unspent)
}
