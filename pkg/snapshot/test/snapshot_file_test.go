//nolint:forcetypeassert,varnamelen,revive,exhaustruct,gosec // we don't care about these linters in test cases
package snapshot_test

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/blang/vfs/memfs"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
)

var protoParams = &iotago.ProtocolParameters{
	Version:       2,
	NetworkName:   "testnet",
	Bech32HRP:     iotago.PrefixTestnet,
	MinPoWScore:   0,
	RentStructure: iotago.RentStructure{},
	BelowMaxDepth: 15,
	TokenSupply:   0,
}

func TestMilestoneDiffSerialization(t *testing.T) {

	type test struct {
		name               string
		testFileName       string
		msDiffGenerator    snapshot.MilestoneDiffProducerFunc
		msDiffGenRetriever msDiffRetrieverFunc
	}

	testCases := []test{
		func() test {
			// generate a milestone diff
			msDiffIterFunc, msDiffGenRetriever := newMsDiffGenerator(1, 50, snapshot.MsDiffDirectionOnwards)

			t := test{
				name:               "ok",
				testFileName:       "test_milestone_diff.bin",
				msDiffGenerator:    msDiffIterFunc,
				msDiffGenRetriever: msDiffGenRetriever,
			}

			return t
		}(),
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {

			filePath := tt.testFileName
			fs := memfs.Create()
			milestoneDiffWrite, err := fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0666)
			require.NoError(t, err)

			writtenMsDiffs := []*snapshot.MilestoneDiff{}

			msDiffCount := 0
			for {
				msDiff, err := tt.msDiffGenerator()
				require.NoError(t, err)

				if msDiff == nil {
					break
				}

				writtenMsDiffs = append(writtenMsDiffs, msDiff)

				msDiffBytes, err := msDiff.MarshalBinary()
				require.NoError(t, err)

				_, err = milestoneDiffWrite.Write(msDiffBytes)
				require.NoError(t, err)
				msDiffCount++
			}
			require.NoError(t, milestoneDiffWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written milestone diff, file size: %s\n", tt.name, humanize.Bytes(uint64(fileInfo.Size())))

			// read back written data and verify that it is equal
			milestoneDiffRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			protocolStorage := getProtocolStorage(protoParams)

			for i := 0; i < msDiffCount; i++ {
				_, msDiff, err := snapshot.ReadMilestoneDiff(milestoneDiffRead, protocolStorage, false)
				require.NoError(t, err)
				writtenMsDiff := writtenMsDiffs[i]

				// verify that what has been written also has been read again
				equalMilestoneDiff(t, writtenMsDiff, msDiff)
			}
		})
	}
}

func TestMilestoneDiffReadProtocolParameters(t *testing.T) {

	type test struct {
		name               string
		testFileName       string
		msDiffGenerator    snapshot.MilestoneDiffProducerFunc
		msDiffGenRetriever msDiffRetrieverFunc
	}

	testCases := []test{
		func() test {
			// generate a milestone diff
			msDiffIterFunc, msDiffGenRetriever := newMsDiffGenerator(1, 50, snapshot.MsDiffDirectionOnwards)

			t := test{
				name:               "ok",
				testFileName:       "test_milestone_diff_read_protocol_pars.bin",
				msDiffGenerator:    msDiffIterFunc,
				msDiffGenRetriever: msDiffGenRetriever,
			}

			return t
		}(),
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {

			filePath := tt.testFileName
			fs := memfs.Create()
			milestoneDiffWrite, err := fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0666)
			require.NoError(t, err)

			writtenProtoParamsMsOptions := []*iotago.ProtocolParamsMilestoneOpt{}

			msDiffCount := 0
			for {
				msDiff, err := tt.msDiffGenerator()
				require.NoError(t, err)

				if msDiff == nil {
					break
				}

				if msDiff.Milestone.Opts.MustSet().ProtocolParams() != nil {
					writtenProtoParamsMsOptions = append(writtenProtoParamsMsOptions, msDiff.Milestone.Opts.MustSet().ProtocolParams())
				}

				msDiffBytes, err := msDiff.MarshalBinary()
				require.NoError(t, err)

				_, err = milestoneDiffWrite.Write(msDiffBytes)
				require.NoError(t, err)
				msDiffCount++
			}
			require.NoError(t, milestoneDiffWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written milestone diff, file size: %s\n", tt.name, humanize.Bytes(uint64(fileInfo.Size())))

			// read back written data and verify that it is equal
			milestoneDiffRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			protocolStorage := getProtocolStorage(nil)

			for i := 0; i < msDiffCount; i++ {
				_, err := snapshot.ReadMilestoneDiffProtocolParameters(milestoneDiffRead, protocolStorage)
				require.NoError(t, err)
			}

			readProtoParamsMsOptions := []*iotago.ProtocolParamsMilestoneOpt{}
			err = protocolStorage.ForEachProtocolParameterMilestoneOption(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
				readProtoParamsMsOptions = append(readProtoParamsMsOptions, protoParamsMsOption)

				return true
			})
			require.NoError(t, err)

			// verify that what has been written also has been read again
			require.EqualValues(t, writtenProtoParamsMsOptions, readProtoParamsMsOptions)
		})
	}
}

func TestStreamFullSnapshotDataToAndFrom(t *testing.T) {
	if testing.Short() {
		return
	}
	rand.Seed(time.Now().Unix())

	type test struct {
		name                          string
		snapshotFileName              string
		originFullHeader              *snapshot.FullSnapshotHeader
		fullHeaderConsumer            snapshot.FullHeaderConsumerFunc
		unspentTreasuryOutputConsumer snapshot.UnspentTreasuryOutputConsumerFunc
		outputGenerator               snapshot.OutputProducerFunc
		outputGenRetriever            outputRetrieverFunc
		outputConsumer                snapshot.OutputConsumerFunc
		outputConRetriever            outputRetrieverFunc
		milestoneDiffsFutureCone      iotago.MilestoneIndex
		msDiffGenerator               snapshot.MilestoneDiffProducerFunc
		msDiffGenRetriever            msDiffRetrieverFunc
		msDiffConsumer                snapshot.MilestoneDiffConsumerFunc
		msDiffConRetriever            msDiffRetrieverFunc
		sepGenerator                  snapshot.SEPProducerFunc
		sepGenRetriever               sepRetrieverFunc
		sepConsumer                   snapshot.SEPConsumerFunc
		sepConRetriever               sepRetrieverFunc
		protoParamsMsOptionsConsumer  snapshot.ProtocolParamsMilestoneOptConsumerFunc
	}

	testCases := []test{
		func() test {
			var milestoneDiffsFutureCone iotago.MilestoneIndex = 10

			originFullHeader := randFullSnapshotHeader(1000000, 50, 150)

			// create generators and consumers
			outputIterFunc, outputGenRetriever := newOutputsGenerator(originFullHeader.OutputCount)
			outputConsumerFunc, outputCollRetriever := newOutputCollector()

			msDiffIterFunc, msDiffGenRetriever := newMsDiffGenerator(originFullHeader.TargetMilestoneIndex+milestoneDiffsFutureCone, originFullHeader.MilestoneDiffCount, snapshot.MsDiffDirectionBackwards)
			msDiffConsumerFunc, msDiffCollRetriever := newMsDiffCollector()

			sepIterFunc, sepGenRetriever := newSEPGenerator(originFullHeader.SEPCount)
			sepConsumerFunc, sepsCollRetriever := newSEPCollector()

			protoParamsMsOptionsConsumerFunc := newProtocolParamsMilestoneOptConsumerFunc()

			t := test{
				name:                          "full: 150 seps, 1 mil outputs, 50 ms diffs",
				snapshotFileName:              "full_snapshot.bin",
				originFullHeader:              originFullHeader,
				fullHeaderConsumer:            fullHeaderEqualFunc(t, originFullHeader),
				unspentTreasuryOutputConsumer: unspentTreasuryOutputEqualFunc(t, originFullHeader.TreasuryOutput),
				outputGenerator:               outputIterFunc,
				outputGenRetriever:            outputGenRetriever,
				outputConsumer:                outputConsumerFunc,
				outputConRetriever:            outputCollRetriever,
				milestoneDiffsFutureCone:      milestoneDiffsFutureCone,
				msDiffGenerator:               msDiffIterFunc,
				msDiffGenRetriever:            msDiffGenRetriever,
				msDiffConsumer:                msDiffConsumerFunc,
				msDiffConRetriever:            msDiffCollRetriever,
				sepGenerator:                  sepIterFunc,
				sepGenRetriever:               sepGenRetriever,
				sepConsumer:                   sepConsumerFunc,
				sepConRetriever:               sepsCollRetriever,
				protoParamsMsOptionsConsumer:  protoParamsMsOptionsConsumerFunc,
			}

			return t
		}(),
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.snapshotFileName
			fs := memfs.Create()
			snapshotFileWrite, err := fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0666)
			require.NoError(t, err)

			_, err = snapshot.StreamFullSnapshotDataTo(snapshotFileWrite, tt.originFullHeader, tt.outputGenerator, tt.msDiffGenerator, tt.sepGenerator)
			require.NoError(t, err)
			require.NoError(t, snapshotFileWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written (snapshot type: %d) snapshot file size: %s\n", tt.name, snapshot.Full, humanize.Bytes(uint64(fileInfo.Size())))

			// read back written data and verify that it is equal
			snapshotFileRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			require.NoError(t, snapshot.StreamFullSnapshotDataFrom(
				context.Background(),
				snapshotFileRead,
				tt.fullHeaderConsumer,
				tt.unspentTreasuryOutputConsumer,
				tt.outputConsumer,
				tt.msDiffConsumer,
				tt.sepConsumer,
				tt.protoParamsMsOptionsConsumer,
			))

			// verify that what has been written also has been read again
			tpkg.EqualOutputs(t, tt.outputGenRetriever(), tt.outputConRetriever())

			msDiffGen := tt.msDiffGenRetriever()
			msDiffCon := tt.msDiffConRetriever()
			require.Equal(t, len(msDiffGen), int(tt.originFullHeader.MilestoneDiffCount))
			require.Equal(t, len(msDiffCon), int(tt.milestoneDiffsFutureCone))

			// in a full snapshot we only consume the milestone diffs with indexes bigger than the target index.
			for i := range msDiffGen {
				gen := msDiffGen[i]
				if i < len(msDiffCon) {
					con := msDiffCon[i]
					require.NotNil(t, con)
					equalMilestoneDiff(t, gen, con)
				}
			}
			require.EqualValues(t, tt.sepGenRetriever(), tt.sepConRetriever())
		})
	}
}

func TestStreamDeltaSnapshotDataToAndFrom(t *testing.T) {
	if testing.Short() {
		return
	}
	rand.Seed(time.Now().Unix())

	type test struct {
		name                         string
		snapshotFileName             string
		originDeltaHeader            *snapshot.DeltaSnapshotHeader
		deltaHeaderConsumer          snapshot.DeltaHeaderConsumerFunc
		msDiffGenerator              snapshot.MilestoneDiffProducerFunc
		msDiffGenRetriever           msDiffRetrieverFunc
		msDiffConsumer               snapshot.MilestoneDiffConsumerFunc
		msDiffConRetriever           msDiffRetrieverFunc
		sepGenerator                 snapshot.SEPProducerFunc
		sepGenRetriever              sepRetrieverFunc
		sepConsumer                  snapshot.SEPConsumerFunc
		sepConRetriever              sepRetrieverFunc
		protoParamsMsOptionsConsumer snapshot.ProtocolParamsMilestoneOptConsumerFunc
	}

	testCases := []test{
		func() test {
			originDeltaHeader := randDeltaSnapshotHeader(50, 150)

			// create generators and consumers
			msDiffIterFunc, msDiffGenRetriever := newMsDiffGenerator(originDeltaHeader.TargetMilestoneIndex-originDeltaHeader.MilestoneDiffCount, originDeltaHeader.MilestoneDiffCount, snapshot.MsDiffDirectionOnwards)
			msDiffConsumerFunc, msDiffCollRetriever := newMsDiffCollector()

			sepIterFunc, sepGenRetriever := newSEPGenerator(originDeltaHeader.SEPCount)
			sepConsumerFunc, sepsCollRetriever := newSEPCollector()

			protoParamsMsOptionsConsumerFunc := newProtocolParamsMilestoneOptConsumerFunc()

			t := test{
				name:                         "delta: 150 seps, 50 ms diffs",
				snapshotFileName:             "delta_snapshot.bin",
				originDeltaHeader:            originDeltaHeader,
				deltaHeaderConsumer:          deltaHeaderEqualFunc(t, originDeltaHeader),
				msDiffGenerator:              msDiffIterFunc,
				msDiffGenRetriever:           msDiffGenRetriever,
				msDiffConsumer:               msDiffConsumerFunc,
				msDiffConRetriever:           msDiffCollRetriever,
				sepGenerator:                 sepIterFunc,
				sepGenRetriever:              sepGenRetriever,
				sepConsumer:                  sepConsumerFunc,
				sepConRetriever:              sepsCollRetriever,
				protoParamsMsOptionsConsumer: protoParamsMsOptionsConsumerFunc,
			}

			return t
		}(),
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.snapshotFileName
			fs := memfs.Create()
			snapshotFileWrite, err := fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0666)
			require.NoError(t, err)

			_, err = snapshot.StreamDeltaSnapshotDataTo(snapshotFileWrite, tt.originDeltaHeader, tt.msDiffGenerator, tt.sepGenerator)
			require.NoError(t, err)
			require.NoError(t, snapshotFileWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written (snapshot type: %d) snapshot file size: %s\n", tt.name, snapshot.Delta, humanize.Bytes(uint64(fileInfo.Size())))

			// read back written data and verify that it is equal
			snapshotFileRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			protocolStorageGetter := func() (*storage.ProtocolStorage, error) {
				return getProtocolStorage(protoParams), nil
			}

			require.NoError(t, snapshot.StreamDeltaSnapshotDataFrom(context.Background(), snapshotFileRead, protocolStorageGetter, tt.deltaHeaderConsumer, tt.msDiffConsumer, tt.sepConsumer, tt.protoParamsMsOptionsConsumer))

			// verify that what has been written also has been read again
			msDiffGen := tt.msDiffGenRetriever()
			msDiffCon := tt.msDiffConRetriever()
			for i := range msDiffGen {
				gen := msDiffGen[i]
				con := msDiffCon[i]
				require.NotNil(t, con)
				equalMilestoneDiff(t, gen, con)
			}
			require.EqualValues(t, tt.sepGenRetriever(), tt.sepConRetriever())
		})
	}
}

func TestStreamDeltaSnapshotDataToExistingAndFrom(t *testing.T) {
	if testing.Short() {
		return
	}
	rand.Seed(time.Now().Unix())

	type test struct {
		name                          string
		snapshotFileName              string
		originDeltaHeader             *snapshot.DeltaSnapshotHeader
		deltaHeaderConsumer           snapshot.DeltaHeaderConsumerFunc
		snapshotExtensionGenerator    deltaSnapshotExtensionGeneratorFunc
		snapshotExtensionGenRetriever deltaSnapshotExtensionRetrieverFunc
		msDiffConsumer                snapshot.MilestoneDiffConsumerFunc
		msDiffConRetriever            msDiffRetrieverFunc
		sepConsumer                   snapshot.SEPConsumerFunc
		sepConRetriever               sepRetrieverFunc
		protoParamsMsOptionsConsumer  snapshot.ProtocolParamsMilestoneOptConsumerFunc
	}

	testCases := []test{
		func() test {
			originDeltaHeader := randDeltaSnapshotHeader(50, 150)

			// create generators and consumers
			snapshotExtensionGenerator, snapshotExtensionGenRetriever := newDeltaSnapshotExtensionGenerator(originDeltaHeader, 10, 50, 30)

			msDiffConsumerFunc, msDiffCollRetriever := newMsDiffCollector()
			sepConsumerFunc, sepsCollRetriever := newSEPCollector()

			protoParamsMsOptionsConsumerFunc := newProtocolParamsMilestoneOptConsumerFunc()

			t := test{
				name:                          "delta: 150 seps, 50 ms diffs",
				snapshotFileName:              "delta_snapshot.bin",
				originDeltaHeader:             originDeltaHeader,
				deltaHeaderConsumer:           deltaHeaderEqualFunc(t, originDeltaHeader),
				snapshotExtensionGenerator:    snapshotExtensionGenerator,
				snapshotExtensionGenRetriever: snapshotExtensionGenRetriever,
				msDiffConsumer:                msDiffConsumerFunc,
				msDiffConRetriever:            msDiffCollRetriever,
				sepConsumer:                   sepConsumerFunc,
				sepConRetriever:               sepsCollRetriever,
				protoParamsMsOptionsConsumer:  protoParamsMsOptionsConsumerFunc,
			}

			return t
		}(),
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.snapshotFileName
			fs := memfs.Create()

			snapshotFileCreated := false
			for {
				msDiffGen, sepGen := tt.snapshotExtensionGenerator()
				if msDiffGen == nil || sepGen == nil {
					break
				}

				if !snapshotFileCreated {

					snapshotFileWrite, err := fs.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0666)
					require.NoError(t, err)

					// create the initial delta snapshot file
					_, err = snapshot.StreamDeltaSnapshotDataTo(snapshotFileWrite, tt.originDeltaHeader, msDiffGen, sepGen)
					require.NoError(t, err)
					require.NoError(t, snapshotFileWrite.Close())

					snapshotFileCreated = true

					continue
				}

				snapshotFileWrite, err := fs.OpenFile(filePath, os.O_RDWR, 0666)
				require.NoError(t, err)

				tt.originDeltaHeader.TargetMilestoneIndex++
				tt.originDeltaHeader.TargetMilestoneTimestamp++

				// extend the existing delta snapshot file
				_, err = snapshot.StreamDeltaSnapshotDataToExisting(snapshotFileWrite, tt.originDeltaHeader, msDiffGen, sepGen)
				require.NoError(t, err)
				require.NoError(t, snapshotFileWrite.Close())
			}

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written (snapshot type: %d) snapshot file size: %s\n", tt.name, snapshot.Delta, humanize.Bytes(uint64(fileInfo.Size())))

			// read back written data and verify that it is equal
			snapshotFileRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			protocolStorageGetter := func() (*storage.ProtocolStorage, error) {
				return getProtocolStorage(protoParams), nil
			}

			require.NoError(t, snapshot.StreamDeltaSnapshotDataFrom(context.Background(), snapshotFileRead, protocolStorageGetter, tt.deltaHeaderConsumer, tt.msDiffConsumer, tt.sepConsumer, tt.protoParamsMsOptionsConsumer))

			// verify that what has been written also has been read again
			msDiffGenRetriever, sepGenRetriever := tt.snapshotExtensionGenRetriever()

			msDiffGen := msDiffGenRetriever()
			msDiffCon := tt.msDiffConRetriever()
			for i := range msDiffGen {
				gen := msDiffGen[i]
				con := msDiffCon[i]
				require.NotNil(t, con)
				equalMilestoneDiff(t, gen, con)
			}
			require.EqualValues(t, sepGenRetriever(), tt.sepConRetriever())
		})
	}
}

type sepRetrieverFunc func() iotago.BlockIDs

func newSEPGenerator(count uint16) (snapshot.SEPProducerFunc, sepRetrieverFunc) {
	var generatedSEPs iotago.BlockIDs

	return func() (iotago.BlockID, error) {
			if count == 0 {
				return iotago.EmptyBlockID(), snapshot.ErrNoMoreSEPToProduce
			}
			count--
			blockID := tpkg.RandBlockID()
			generatedSEPs = append(generatedSEPs, blockID)

			return blockID, nil
		}, func() iotago.BlockIDs {
			return generatedSEPs
		}
}

func newSEPCollector() (snapshot.SEPConsumerFunc, sepRetrieverFunc) {
	var generatedSEPs iotago.BlockIDs

	return func(sep iotago.BlockID, targetMilestoneIndex iotago.MilestoneIndex) error {
			generatedSEPs = append(generatedSEPs, sep)

			return nil
		}, func() iotago.BlockIDs {
			return generatedSEPs
		}
}

func newProtocolParamsMilestoneOptConsumerFunc() snapshot.ProtocolParamsMilestoneOptConsumerFunc {
	// we check for duplicated entries in the protocol parameter milestone options
	existingProtoParamsMsOpts := make(map[iotago.MilestoneIndex]struct{})

	return func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) error {
		if _, exists := existingProtoParamsMsOpts[protoParamsMsOption.TargetMilestoneIndex]; exists {
			return storage.ErrProtocolParamsMilestoneOptAlreadyExists
		}

		return nil
	}
}

type outputRetrieverFunc func() utxo.Outputs

func newOutputsGenerator(count uint64) (snapshot.OutputProducerFunc, outputRetrieverFunc) {
	var generatedOutputs utxo.Outputs

	return func() (*utxo.Output, error) {
			if count == 0 {
				return nil, nil
			}
			count--
			output := tpkg.RandUTXOOutput()
			generatedOutputs = append(generatedOutputs, output)

			return output, nil
		}, func() utxo.Outputs {
			return generatedOutputs
		}
}

func newOutputCollector() (snapshot.OutputConsumerFunc, outputRetrieverFunc) {
	var generatedOutputs utxo.Outputs

	return func(o *utxo.Output) error {
			generatedOutputs = append(generatedOutputs, o)

			return nil
		}, func() utxo.Outputs {
			return generatedOutputs
		}
}

type msDiffRetrieverFunc func() []*snapshot.MilestoneDiff

func newMsDiffGenerator(startIndex iotago.MilestoneIndex, count syncmanager.MilestoneIndexDelta, direction snapshot.MsDiffDirection) (snapshot.MilestoneDiffProducerFunc, msDiffRetrieverFunc) {
	var generatedMsDiffs []*snapshot.MilestoneDiff
	pub, prv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}

	var mappingPubKey iotago.MilestonePublicKey
	copy(mappingPubKey[:], pub)
	pubKeys := []iotago.MilestonePublicKey{mappingPubKey}

	keyMapping := iotago.MilestonePublicKeyMapping{}
	keyMapping[mappingPubKey] = prv

	targetIndex := startIndex + count
	if direction == snapshot.MsDiffDirectionBackwards {
		targetIndex = startIndex - count
	}

	msIterator := snapshot.NewMsIndexIterator(direction, startIndex, targetIndex)

	return func() (*snapshot.MilestoneDiff, error) {

			milestoneIndex, done := msIterator()
			if done {
				return nil, nil
			}

			parents := iotago.BlockIDs{tpkg.RandBlockID()}
			milestonePayload := iotago.NewMilestone(milestoneIndex, tpkg.RandMilestoneTimestamp(), protoParams.Version, tpkg.RandMilestoneID(), parents, tpkg.Rand32ByteHash(), tpkg.Rand32ByteHash())

			receipt, err := tpkg.RandReceipt(milestonePayload.Index, protoParams)
			if err != nil {
				panic(err)
			}

			newProtoParamsBytes, err := tpkg.RandProtocolParameters().Serialize(serializer.DeSeriModePerformValidation, nil)
			if err != nil {
				panic(err)
			}

			protoParamsMsOption := &iotago.ProtocolParamsMilestoneOpt{
				TargetMilestoneIndex: milestonePayload.Index + 15,
				ProtocolVersion:      tpkg.RandByte(),
				Params:               newProtoParamsBytes,
			}

			milestonePayload.Opts = iotago.MilestoneOpts{receipt, protoParamsMsOption}

			if err := milestonePayload.Sign(pubKeys, iotago.InMemoryEd25519MilestoneSigner(keyMapping)); err != nil {
				panic(err)
			}

			msDiff := &snapshot.MilestoneDiff{
				Milestone: milestonePayload,
			}

			createdCount := rand.Intn(500) + 1
			for i := 0; i < createdCount; i++ {
				msDiff.Created = append(msDiff.Created, tpkg.RandUTXOOutput())
			}

			consumedCount := rand.Intn(500) + 1
			for i := 0; i < consumedCount; i++ {
				msDiff.Consumed = append(msDiff.Consumed, tpkg.RandUTXOSpent(milestonePayload.Index, milestonePayload.Timestamp))
			}

			msDiff.SpentTreasuryOutput = &utxo.TreasuryOutput{
				MilestoneID: tpkg.RandMilestoneID(),
				Amount:      tpkg.RandAmount(),
				Spent:       true, // doesn't matter
			}

			generatedMsDiffs = append(generatedMsDiffs, msDiff)

			return msDiff, nil
		}, func() []*snapshot.MilestoneDiff {
			return generatedMsDiffs
		}
}

func newMsDiffCollector() (snapshot.MilestoneDiffConsumerFunc, msDiffRetrieverFunc) {
	var generatedMsDiffs []*snapshot.MilestoneDiff

	return func(msDiff *snapshot.MilestoneDiff) error {
			generatedMsDiffs = append(generatedMsDiffs, msDiff)

			return nil
		}, func() []*snapshot.MilestoneDiff {
			return generatedMsDiffs
		}
}

type deltaSnapshotExtensionGeneratorFunc func() (snapshot.MilestoneDiffProducerFunc, snapshot.SEPProducerFunc)
type deltaSnapshotExtensionRetrieverFunc func() (msDiffRetrieverFunc, sepRetrieverFunc)

func newDeltaSnapshotExtensionGenerator(deltaHeader *snapshot.DeltaSnapshotHeader, extensionsCount uint16, msDiffCount uint32, sepCount uint16) (deltaSnapshotExtensionGeneratorFunc, deltaSnapshotExtensionRetrieverFunc) {
	var generatedSEPs iotago.BlockIDs
	var generatedMsDiffs []*snapshot.MilestoneDiff

	startMilestoneIndex := deltaHeader.TargetMilestoneIndex

	return func() (snapshot.MilestoneDiffProducerFunc, snapshot.SEPProducerFunc) {
			if extensionsCount == 0 {
				return nil, nil
			}
			extensionsCount--

			msDiffIterFunc, _ := newMsDiffGenerator(startMilestoneIndex-msDiffCount, msDiffCount, snapshot.MsDiffDirectionOnwards)
			sepIterFunc, _ := newSEPGenerator(sepCount)

			// reset the SEP every time
			generatedSEPs = make(iotago.BlockIDs, 0)

			return func() (*snapshot.MilestoneDiff, error) {
					msDiff, err := msDiffIterFunc()
					if err != nil {
						return nil, err
					}
					if msDiff == nil {
						return nil, nil
					}
					startMilestoneIndex++
					generatedMsDiffs = append(generatedMsDiffs, msDiff)

					return msDiff, nil
				}, func() (iotago.BlockID, error) {

					sep, err := sepIterFunc()
					if err != nil {
						return iotago.EmptyBlockID(), err
					}
					generatedSEPs = append(generatedSEPs, sep)

					return sep, nil
				}
		}, func() (msDiffRetrieverFunc, sepRetrieverFunc) {
			return func() []*snapshot.MilestoneDiff {
					return generatedMsDiffs
				}, func() iotago.BlockIDs {
					return generatedSEPs
				}
		}
}

func fullHeaderEqualFunc(t *testing.T, expected *snapshot.FullSnapshotHeader) snapshot.FullHeaderConsumerFunc {
	return func(actual *snapshot.FullSnapshotHeader) error {
		require.EqualValues(t, expected.Version, actual.Version)
		require.EqualValues(t, expected.Type, actual.Type)
		require.EqualValues(t, expected.GenesisMilestoneIndex, actual.GenesisMilestoneIndex)
		require.EqualValues(t, expected.TargetMilestoneIndex, actual.TargetMilestoneIndex)
		require.EqualValues(t, expected.TargetMilestoneTimestamp, actual.TargetMilestoneTimestamp)
		require.EqualValues(t, expected.TargetMilestoneID, actual.TargetMilestoneID)
		require.EqualValues(t, expected.LedgerMilestoneIndex, actual.LedgerMilestoneIndex)
		require.EqualValues(t, expected.TreasuryOutput, actual.TreasuryOutput)
		require.EqualValues(t, expected.ProtocolParamsMilestoneOpt, actual.ProtocolParamsMilestoneOpt)
		require.EqualValues(t, expected.OutputCount, actual.OutputCount)
		require.EqualValues(t, expected.MilestoneDiffCount, actual.MilestoneDiffCount)
		require.EqualValues(t, expected.SEPCount, actual.SEPCount)

		return nil
	}
}

func deltaHeaderEqualFunc(t *testing.T, expected *snapshot.DeltaSnapshotHeader) snapshot.DeltaHeaderConsumerFunc {
	return func(actual *snapshot.DeltaSnapshotHeader) error {
		require.EqualValues(t, expected.Version, actual.Version)
		require.EqualValues(t, expected.Type, actual.Type)
		require.EqualValues(t, expected.TargetMilestoneIndex, actual.TargetMilestoneIndex)
		require.EqualValues(t, expected.TargetMilestoneTimestamp, actual.TargetMilestoneTimestamp)
		require.EqualValues(t, expected.FullSnapshotTargetMilestoneID, actual.FullSnapshotTargetMilestoneID)
		require.EqualValues(t, expected.SEPFileOffset, actual.SEPFileOffset)
		require.EqualValues(t, expected.MilestoneDiffCount, actual.MilestoneDiffCount)
		require.EqualValues(t, expected.SEPCount, actual.SEPCount)

		return nil
	}
}

func unspentTreasuryOutputEqualFunc(t *testing.T, originUnspentTreasuryOutput *utxo.TreasuryOutput) snapshot.UnspentTreasuryOutputConsumerFunc {
	return func(readUnspentTreasuryOutput *utxo.TreasuryOutput) error {
		require.EqualValues(t, *originUnspentTreasuryOutput, *readUnspentTreasuryOutput)

		return nil
	}
}

func randFullSnapshotHeader(outputCount uint64, msDiffCount uint32, sepCount uint16) *snapshot.FullSnapshotHeader {

	targetMilestoneIndex := tpkg.RandMilestoneIndex()
	for targetMilestoneIndex < msDiffCount+1 {
		targetMilestoneIndex = tpkg.RandMilestoneIndex()
	}

	return &snapshot.FullSnapshotHeader{
		Type:                       snapshot.Full,
		Version:                    snapshot.SupportedFormatVersion,
		GenesisMilestoneIndex:      tpkg.RandMilestoneIndex(),
		TargetMilestoneIndex:       targetMilestoneIndex,
		TargetMilestoneTimestamp:   tpkg.RandMilestoneTimestamp(),
		TargetMilestoneID:          tpkg.RandMilestoneID(),
		LedgerMilestoneIndex:       tpkg.RandMilestoneIndex(),
		TreasuryOutput:             tpkg.RandTreasuryOutput(),
		ProtocolParamsMilestoneOpt: tpkg.RandProtocolParamsMilestoneOpt(targetMilestoneIndex),
		OutputCount:                outputCount,
		MilestoneDiffCount:         msDiffCount,
		SEPCount:                   sepCount,
	}
}

func randDeltaSnapshotHeader(msDiffCount uint32, sepCount uint16) *snapshot.DeltaSnapshotHeader {
	return &snapshot.DeltaSnapshotHeader{
		Version:                       snapshot.SupportedFormatVersion,
		Type:                          snapshot.Delta,
		TargetMilestoneIndex:          tpkg.RandMilestoneIndex(),
		TargetMilestoneTimestamp:      tpkg.RandMilestoneTimestamp(),
		FullSnapshotTargetMilestoneID: tpkg.RandMilestoneID(),
		SEPFileOffset:                 0,
		MilestoneDiffCount:            msDiffCount,
		SEPCount:                      sepCount,
	}
}

func getProtocolStorage(protoParams *iotago.ProtocolParameters) *storage.ProtocolStorage {

	// initialize a temporary protocol storage in memory
	protocolStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

	if protoParams != nil {
		// add initial protocol parameters to the protocol storage

		protoParamsBytes, err := protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			panic(err)
		}

		// write the protocol parameters to the storage
		err = protocolStorage.StoreProtocolParametersMilestoneOption(
			&iotago.ProtocolParamsMilestoneOpt{
				TargetMilestoneIndex: 0,
				ProtocolVersion:      protoParams.Version,
				Params:               protoParamsBytes,
			},
		)
		if err != nil {
			panic(err)
		}
	}

	return protocolStorage
}

func equalMilestoneDiff(t *testing.T, expected *snapshot.MilestoneDiff, actual *snapshot.MilestoneDiff) {
	require.EqualValues(t, expected.Milestone, actual.Milestone)
	require.EqualValues(t, expected.SpentTreasuryOutput, actual.SpentTreasuryOutput)
	tpkg.EqualOutputs(t, expected.Created, actual.Created)
	tpkg.EqualSpents(t, expected.Consumed, actual.Consumed)
}
