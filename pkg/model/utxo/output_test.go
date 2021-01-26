package utxo

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
)

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func EqualOutputs(t *testing.T, expected *Output, actual *Output) {
	require.True(t, bytes.Equal(expected.OutputID()[:], actual.OutputID()[:]))
	require.True(t, bytes.Equal(expected.MessageID()[:], actual.MessageID()[:]))
	require.Equal(t, expected.OutputType(), actual.OutputType())
	require.Equal(t, expected.Address().String(), actual.Address().String())
	require.Equal(t, expected.Amount(), actual.Amount())
}

func TestOutputSerialization(t *testing.T) {

	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], randBytes(34))

	messageID := &hornet.MessageID{}
	copy(messageID[:], randBytes(32))

	outputType := iotago.OutputSigLockedDustAllowanceOutput

	address := &iotago.Ed25519Address{}
	addressBytes := randBytes(32)
	copy(address[:], addressBytes)

	amount := uint64(832493)

	output := CreateOutput(outputID, messageID, outputType, address, amount)

	require.True(t, bytes.Equal(byteutils.ConcatBytes([]byte{iotago.AddressEd25519}, addressBytes), output.addressBytes()))

	require.True(t, bytes.Equal(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, outputID[:]), output.kvStorableKey()))

	value := output.kvStorableValue()
	require.True(t, bytes.Equal(messageID[:], value[:32]))
	require.Equal(t, outputType, value[32])
	require.Equal(t, amount, binary.LittleEndian.Uint64(value[33:41]))
	require.Equal(t, iotago.AddressEd25519, value[41])
	require.True(t, bytes.Equal(addressBytes, value[42:74]))

	require.True(t, bytes.Equal(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixSpent}, []byte{iotago.AddressEd25519}, addressBytes, []byte{outputType}, outputID[:]), output.spentDatabaseKey()))
	require.True(t, bytes.Equal(byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixUnspent}, []byte{iotago.AddressEd25519}, addressBytes, []byte{outputType}, outputID[:]), output.unspentDatabaseKey()))

	input := output.UTXOInput()
	require.True(t, bytes.Equal(outputID[:32], input.TransactionID[:]))
	require.Equal(t, binary.LittleEndian.Uint16(outputID[32:34]), input.TransactionOutputIndex)

	store := mapdb.NewMapDB()

	utxo := New(store)

	require.NoError(t, utxo.AddUnspentOutput(output))

	readOutput, err := utxo.ReadOutputByOutputID(outputID)
	require.NoError(t, err)

	EqualOutputs(t, output, readOutput)

}
