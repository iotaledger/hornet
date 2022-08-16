//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package snapshot_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestSnapshotOutputProducerAndConsumer(t *testing.T) {
	map1 := mapdb.NewMapDB()
	u1 := utxo.New(map1)
	map2 := mapdb.NewMapDB()
	u2 := utxo.New(map2)

	count := 1000

	// Fill up the UTXO
	var err error
	for i := 0; i < count; i++ {
		err = u1.AddUnspentOutput(tpkg.RandUTXOOutputWithType(iotago.OutputBasic))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(tpkg.RandUTXOOutputWithType(iotago.OutputAlias))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(tpkg.RandUTXOOutputWithType(iotago.OutputNFT))
		require.NoError(t, err)

		err = u1.AddUnspentOutput(tpkg.RandUTXOOutputWithType(iotago.OutputFoundry))
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
	producer := snapshot.NewCMIUTXOProducer(u1)
	consumer := snapshot.NewOutputConsumer(u2)

	for {
		output, err := producer()
		require.NoError(t, err)

		if output == nil {
			break
		}

		// Marshal the output
		outputBytes := output.SnapshotBytes()

		// Unmarshal the output again
		newOutput, err := snapshot.ReadOutput(bytes.NewReader(outputBytes), nil)
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

	var startIndex iotago.MilestoneIndex = 1000
	var targetIndex iotago.MilestoneIndex = 1050
	msIterator := snapshot.NewMsIndexIterator(snapshot.MsDiffDirectionOnwards, startIndex, targetIndex)

	var done bool
	var msIndex iotago.MilestoneIndex

	currentIndex := startIndex + 1
	for msIndex, done = msIterator(); !done; msIndex, done = msIterator() {
		require.GreaterOrEqual(t, msIndex, startIndex+1)
		require.LessOrEqual(t, msIndex, targetIndex)
		require.Equal(t, currentIndex, msIndex)
		currentIndex++
	}
}

func TestMsIndexIteratorBackwards(t *testing.T) {

	var startIndex iotago.MilestoneIndex = 1050
	var targetIndex iotago.MilestoneIndex = 1000
	msIterator := snapshot.NewMsIndexIterator(snapshot.MsDiffDirectionBackwards, startIndex, targetIndex)

	var done bool
	var msIndex iotago.MilestoneIndex

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
	var startIndex iotago.MilestoneIndex = 1000
	var targetIndex iotago.MilestoneIndex = 1050
	msIterator := snapshot.NewMsIndexIterator(snapshot.MsDiffDirectionOnwards, startIndex, targetIndex)

	var done bool
	var msIndex iotago.MilestoneIndex

	for msIndex, done = msIterator(); !done; msIndex, done = msIterator() {

		outputs := utxo.Outputs{
			tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
			tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
			tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
			tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
			tpkg.RandUTXOOutputWithType(iotago.OutputBasic),
		}

		spents := utxo.Spents{
			tpkg.RandUTXOSpentWithOutput(outputs[3], msIndex, tpkg.RandMilestoneTimestamp()),
			tpkg.RandUTXOSpentWithOutput(outputs[2], msIndex, tpkg.RandMilestoneTimestamp()),
		}

		require.NoError(t, u1.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))
	}

	producerU1 := snapshot.NewMsDiffsProducer(func(index iotago.MilestoneIndex) (*iotago.Milestone, error) {
		return &iotago.Milestone{Index: index}, nil
	}, u1, snapshot.MsDiffDirectionOnwards, startIndex, targetIndex)
	consumerU2 := snapshot.NewMsDiffConsumer(nil, u2, false)

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
	tpkg.EqualOutputs(t, loadedUnspentU1, loadedUnspentU2)
	tpkg.EqualSpents(t, loadedSpentsU1, loadedSpentsU2)

	// Compare the raw keys values in the backing store
	err = map1.Iterate(kvstore.EmptyPrefix, func(key kvstore.Key, value kvstore.Value) bool {

		value2, err := map2.Get(key)
		require.NoError(t, err)
		require.Equal(t, value, value2)

		return true
	})
	require.NoError(t, err)
}
