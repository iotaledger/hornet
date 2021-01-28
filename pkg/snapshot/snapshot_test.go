package snapshot

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/utxo"
)

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func randomAddress() *iotago.Ed25519Address {
	address := &iotago.Ed25519Address{}
	addressBytes := randBytes(32)
	copy(address[:], addressBytes)
	return address
}

func randomOutput(outputType iotago.OutputType, address ...iotago.Address) *utxo.Output {
	outputID := &iotago.UTXOInputID{}
	copy(outputID[:], randBytes(34))

	messageID := &hornet.MessageID{}
	copy(messageID[:], randBytes(32))

	var addr iotago.Address
	if len(address) > 0 {
		addr = address[0]
	} else {
		addr = randomAddress()
	}

	amount := uint64(rand.Intn(2156465))

	return utxo.CreateOutput(outputID, messageID, outputType, addr, amount)
}

func TestSnapshotOutputProducerAndConsumer(t *testing.T) {

	map1 := mapdb.NewMapDB()
	u1 := utxo.New(map1)
	map2 := mapdb.NewMapDB()
	u2 := utxo.New(map2)

	count := 5000

	// Fill up the UTXO
	for i := 0; i < count; i++ {
		u1.AddUnspentOutput(randomOutput(iotago.OutputSigLockedSingleOutput))
		u1.AddUnspentOutput(randomOutput(iotago.OutputSigLockedDustAllowanceOutput))
	}

	// Count the outputs in the ledger
	var singleCount int
	var allowanceCount int
	u1.ForEachOutput(func(output *utxo.Output) bool {
		switch output.OutputType() {
		case iotago.OutputSigLockedSingleOutput:
			singleCount++
		case iotago.OutputSigLockedDustAllowanceOutput:
			allowanceCount++
		default:
			require.Fail(t, "invalid output type")
		}
		return true
	})
	require.Equal(t, count, singleCount)
	require.Equal(t, count, allowanceCount)

	// Pass all outputs from u1 to u2 over the snapshot serialization functions
	producer := newLSMIUTXOProducer(u1)
	consumer := newOutputConsumer(u2)

	for {
		output, err := producer()
		require.NoError(t, err)

		if output == nil {
			break
		}

		// Marshal the output
		outputBytes, err := output.MarshalBinary()
		require.NoError(t, err)

		// Unmarshal the output again
		newOutput, err := readOutput(bytes.NewBuffer(outputBytes))
		require.NoError(t, err)

		err = consumer(newOutput)
		require.NoError(t, err)
	}

	// Compare the raw keys values in the backing store
	err := map1.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {

		value2, err := map2.Get(key)
		require.NoError(t, err)
		require.Equal(t, value, value2)

		return true
	})
	require.NoError(t, err)

	// Count the outputs in the new ledger
	singleCount = 0
	allowanceCount = 0
	u2.ForEachOutput(func(output *utxo.Output) bool {
		switch output.OutputType() {
		case iotago.OutputSigLockedSingleOutput:
			singleCount++
		case iotago.OutputSigLockedDustAllowanceOutput:
			allowanceCount++
		default:
			require.Fail(t, "invalid output type")
		}
		return true
	})
	require.Equal(t, count, singleCount)
	require.Equal(t, count, allowanceCount)
}
