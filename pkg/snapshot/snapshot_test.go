package snapshot

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/model/utxo/utils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v3"
)

func randomOutput(outputType iotago.OutputType, address ...iotago.Address) *utxo.Output {
	var output iotago.Output
	if len(address) > 0 {
		output = utils.RandOutputOnAddress(outputType, address[0])
	} else {
		output = utils.RandOutput(outputType)
	}
	return utxo.CreateOutput(utils.RandOutputID(), utils.RandMessageID(), utils.RandMilestoneIndex(), rand.Uint64(), output)
}

func randomSpent(output *utxo.Output, msIndex ...milestone.Index) *utxo.Spent {
	confirmationIndex := utils.RandMilestoneIndex()
	if len(msIndex) > 0 {
		confirmationIndex = msIndex[0]
	}
	return utxo.NewSpent(output, utils.RandTransactionID(), confirmationIndex, rand.Uint64())
}

func EqualOutput(t *testing.T, expected *utxo.Output, actual *utxo.Output) {
	require.Equal(t, expected.OutputID()[:], actual.OutputID()[:])
	require.Equal(t, expected.MessageID()[:], actual.MessageID()[:])
	require.Equal(t, expected.OutputType(), actual.OutputType())

	var expectedIdent iotago.Address
	switch output := expected.Output().(type) {
	case iotago.TransIndepIdentOutput:
		expectedIdent = output.Ident()
	case iotago.TransDepIdentOutput:
		expectedIdent = output.Chain().ToAddress()
	default:
		require.Fail(t, "unsupported output type")
	}

	var actualIdent iotago.Address
	switch output := actual.Output().(type) {
	case iotago.TransIndepIdentOutput:
		actualIdent = output.Ident()
	case iotago.TransDepIdentOutput:
		actualIdent = output.Chain().ToAddress()
	default:
		require.Fail(t, "unsupported output type")
	}

	require.True(t, expectedIdent.Equal(actualIdent))
	require.Equal(t, expected.Deposit(), actual.Deposit())
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
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
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

	count := 1000

	// Fill up the UTXO
	var err error
	for i := 0; i < count; i++ {
		err = u1.AddUnspentOutput(randomOutput(iotago.OutputBasic))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(randomOutput(iotago.OutputAlias))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(randomOutput(iotago.OutputNFT))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(randomOutput(iotago.OutputFoundry))
		require.NoError(t, err)
	}

	// Count the outputs in the ledger
	var basicCount int
	var nftCount int
	var foundryCount int
	var aliasCount int
	err = u1.ForEachOutput(func(output *utxo.Output) bool {
		switch output.OutputType() {
		case iotago.OutputBasic:
			basicCount++
		case iotago.OutputNFT:
			nftCount++
		case iotago.OutputFoundry:
			foundryCount++
		case iotago.OutputAlias:
			aliasCount++
		default:
			require.Fail(t, "invalid output type")
		}
		return true
	})
	require.NoError(t, err)
	require.Equal(t, count, basicCount)
	require.Equal(t, count, nftCount)
	require.Equal(t, count, foundryCount)
	require.Equal(t, count, aliasCount)

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
		outputBytes := output.SnapshotBytes()

		// Unmarshal the output again
		newOutput, err := readOutput(bytes.NewReader(outputBytes), iotago.ZeroRentParas)
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
	basicCount = 0
	nftCount = 0
	foundryCount = 0
	aliasCount = 0
	err = u2.ForEachOutput(func(output *utxo.Output) bool {
		switch output.OutputType() {
		case iotago.OutputBasic:
			basicCount++
		case iotago.OutputNFT:
			nftCount++
		case iotago.OutputFoundry:
			foundryCount++
		case iotago.OutputAlias:
			aliasCount++
		default:
			require.Fail(t, "invalid output type")
		}
		return true
	})
	require.NoError(t, err)
	require.Equal(t, count, basicCount)
	require.Equal(t, count, nftCount)
	require.Equal(t, count, foundryCount)
	require.Equal(t, count, aliasCount)
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
			randomOutput(iotago.OutputBasic),
			randomOutput(iotago.OutputBasic),
			randomOutput(iotago.OutputBasic),
			randomOutput(iotago.OutputBasic),
			randomOutput(iotago.OutputBasic),
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
