package snapshot

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v2"
)

// returns length amount random bytes
func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func randMessageID() hornet.MessageID {
	return hornet.MessageID(randBytes(iotago.MessageIDLength))
}

func randomAddress() *iotago.Ed25519Address {
	address := &iotago.Ed25519Address{}
	addressBytes := randBytes(32)
	copy(address[:], addressBytes)
	return address
}

//nolint:unparam // maybe address will be used in the future
func randomOutput(outputType iotago.OutputType, address ...iotago.Address) *utxo.Output {
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

	return utxo.CreateOutput(outputID, messageID, outputType, addr, amount)
}

func randomSpent(output *utxo.Output, msIndex ...milestone.Index) *utxo.Spent {
	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], randBytes(iotago.TransactionIDLength))

	confirmationIndex := milestone.Index(rand.Intn(216589))
	if len(msIndex) > 0 {
		confirmationIndex = msIndex[0]
	}

	return utxo.NewSpent(output, transactionID, confirmationIndex)
}

func EqualOutput(t *testing.T, expected *utxo.Output, actual *utxo.Output) {
	require.Equal(t, expected.OutputID()[:], actual.OutputID()[:])
	require.Equal(t, expected.MessageID()[:], actual.MessageID()[:])
	require.Equal(t, expected.OutputType(), actual.OutputType())
	require.Equal(t, expected.Address().String(), actual.Address().String())
	require.Equal(t, expected.Amount(), actual.Amount())
}

func EqualOutputs(t *testing.T, expected utxo.Outputs, actual utxo.Outputs) {
	require.Equal(t, len(expected), len(actual))

	// Sort Outputs by output ID.
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(expected[i].OutputID()[:], expected[j].OutputID()[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		return bytes.Compare(actual[i].OutputID()[:], actual[j].OutputID()[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualOutput(t, expected[i], actual[i])
	}
}

func EqualSpent(t *testing.T, expected *utxo.Spent, actual *utxo.Spent) {
	require.Equal(t, expected.OutputID()[:], actual.OutputID()[:])
	require.Equal(t, expected.TargetTransactionID()[:], actual.TargetTransactionID()[:])
	require.Equal(t, expected.ConfirmationIndex(), actual.ConfirmationIndex())
	EqualOutput(t, expected.Output(), actual.Output())
}

func EqualSpents(t *testing.T, expected utxo.Spents, actual utxo.Spents) {
	require.Equal(t, len(expected), len(actual))

	// Sort Spents by output ID.
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(expected[i].OutputID()[:], expected[j].OutputID()[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		return bytes.Compare(actual[i].OutputID()[:], actual[j].OutputID()[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualSpent(t, expected[i], actual[i])
	}
}

func TestSnapshotOutputProducerAndConsumer(t *testing.T) {

	map1 := mapdb.NewMapDB()
	u1 := utxo.New(map1)
	map2 := mapdb.NewMapDB()
	u2 := utxo.New(map2)

	count := 5000

	// Fill up the UTXO
	var err error
	for i := 0; i < count; i++ {
		err = u1.AddUnspentOutput(randomOutput(iotago.OutputSigLockedSingleOutput))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(randomOutput(iotago.OutputSigLockedDustAllowanceOutput))
		require.NoError(t, err)
	}

	// Count the outputs in the ledger
	var singleCount int
	var allowanceCount int
	err = u1.ForEachOutput(func(output *utxo.Output) bool {
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
	require.NoError(t, err)
	require.Equal(t, count, singleCount)
	require.Equal(t, count, allowanceCount)

	// Pass all outputs from u1 to u2 over the snapshot serialization functions
	producer := newCMIUTXOProducer(u1)
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
	err = map1.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {

		value2, err := map2.Get(key)
		require.NoError(t, err)
		require.Equal(t, value, value2)

		return true
	})
	require.NoError(t, err)

	// Count the outputs in the new ledger
	singleCount = 0
	allowanceCount = 0
	err = u2.ForEachOutput(func(output *utxo.Output) bool {
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
	require.NoError(t, err)
	require.Equal(t, count, singleCount)
	require.Equal(t, count, allowanceCount)
}

func TestMsIndexIteratorOnwards(t *testing.T) {

	var startIndex milestone.Index = 1000
	var targetIndex milestone.Index = 1050
	msIterator := newMsIndexIterator(MsDiffDirectionOnwards, startIndex, targetIndex)

	var done bool
	var msIndex milestone.Index

	currentIndex := startIndex + 1
	for msIndex, done = msIterator(); !done; msIndex, done = msIterator() {
		require.GreaterOrEqual(t, msIndex, startIndex+1)
		require.LessOrEqual(t, msIndex, targetIndex)
		require.Equal(t, currentIndex, msIndex)
		currentIndex++
	}
}

func TestMsIndexIteratorBackwards(t *testing.T) {

	var startIndex milestone.Index = 1050
	var targetIndex milestone.Index = 1000
	msIterator := newMsIndexIterator(MsDiffDirectionBackwards, startIndex, targetIndex)

	var done bool
	var msIndex milestone.Index

	currentIndex := startIndex
	for msIndex, done = msIterator(); !done; msIndex, done = msIterator() {
		require.GreaterOrEqual(t, msIndex, targetIndex+1)
		require.LessOrEqual(t, msIndex, startIndex)
		require.Equal(t, currentIndex, msIndex)
		currentIndex--
	}
}

func TestSnapshotMsDiffProducerAndConsumer(t *testing.T) {

	map1 := mapdb.NewMapDB()
	u1 := utxo.New(map1)
	map2 := mapdb.NewMapDB()
	u2 := utxo.New(map2)

	// fill the first UTXO manager with some data
	var startIndex milestone.Index = 1000
	var targetIndex milestone.Index = 1050
	msIterator := newMsIndexIterator(MsDiffDirectionOnwards, startIndex, targetIndex)

	var done bool
	var msIndex milestone.Index

	for msIndex, done = msIterator(); !done; msIndex, done = msIterator() {

		outputs := utxo.Outputs{
			randomOutput(iotago.OutputSigLockedSingleOutput),
			randomOutput(iotago.OutputSigLockedSingleOutput),
			randomOutput(iotago.OutputSigLockedDustAllowanceOutput),
			randomOutput(iotago.OutputSigLockedSingleOutput),
			randomOutput(iotago.OutputSigLockedSingleOutput),
		}

		spents := utxo.Spents{
			randomSpent(outputs[3], msIndex),
			randomSpent(outputs[2], msIndex),
		}

		require.NoError(t, u1.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))
	}

	producerU1 := newMsDiffsProducer(func(index milestone.Index) (*iotago.Milestone, error) {
		return &iotago.Milestone{Index: uint32(index)}, nil
	}, u1, MsDiffDirectionOnwards, startIndex, targetIndex)
	consumerU2 := newMsDiffConsumer(u2)

	err := u2.StoreLedgerIndex(startIndex)
	require.NoError(t, err)

	// produce milestone diffs from UTXO manager 1 and apply them to UTXO manager 2 by consuming
	for {
		msDiff, err := producerU1()
		require.NoError(t, err)

		if msDiff == nil {
			break
		}

		err = consumerU2(msDiff)
		require.NoError(t, err)
	}

	//
	loadedSpentsU1, err := u1.SpentOutputs()
	require.NoError(t, err)

	loadedUnspentU1, err := u1.UnspentOutputs()
	require.NoError(t, err)

	loadedSpentsU2, err := u2.SpentOutputs()
	require.NoError(t, err)

	loadedUnspentU2, err := u2.UnspentOutputs()
	require.NoError(t, err)

	// Compare all Outputs and Spents
	EqualOutputs(t, loadedUnspentU1, loadedUnspentU2)
	EqualSpents(t, loadedSpentsU1, loadedSpentsU2)

	// Compare the raw keys values in the backing store
	err = map1.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {

		value2, err := map2.Get(key)
		require.NoError(t, err)
		require.Equal(t, value, value2)

		return true
	})
	require.NoError(t, err)
}
