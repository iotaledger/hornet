package snapshot_test

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/blang/vfs/memfs"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/model/utxo/utils"
	"github.com/iotaledger/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

type test struct {
	name                          string
	snapshotFileName              string
	originHeader                  *snapshot.FileHeader
	originTimestamp               uint32
	sepGenerator                  snapshot.SEPProducerFunc
	sepGenRetriever               sepRetrieverFunc
	outputGenerator               snapshot.OutputProducerFunc
	outputGenRetriever            outputRetrieverFunc
	msDiffGenerator               snapshot.MilestoneDiffProducerFunc
	msDiffGenRetriever            msDiffRetrieverFunc
	headerConsumer                snapshot.HeaderConsumerFunc
	sepConsumer                   snapshot.SEPConsumerFunc
	sepConRetriever               sepRetrieverFunc
	outputConsumer                snapshot.OutputConsumerFunc
	outputConRetriever            outputRetrieverFunc
	unspentTreasuryOutputConsumer snapshot.UnspentTreasuryOutputConsumerFunc
	msDiffConsumer                snapshot.MilestoneDiffConsumerFunc
	msDiffConRetriever            msDiffRetrieverFunc
}

var protoParas = &iotago.ProtocolParameters{
	Version:       2,
	NetworkName:   "testnet",
	Bech32HRP:     iotago.PrefixTestnet,
	MinPoWScore:   0,
	RentStructure: iotago.RentStructure{},
	BelowMaxDepth: 15,
	TokenSupply:   0,
}

func TestStreamLocalSnapshotDataToAndFrom(t *testing.T) {
	if testing.Short() {
		return
	}
	rand.Seed(346587549867)

	testCases := []test{
		func() test {
			originHeader := &snapshot.FileHeader{
				Type:                 snapshot.Full,
				Version:              snapshot.SupportedFormatVersion,
				NetworkID:            1337133713371337,
				SEPMilestoneIndex:    milestone.Index(rand.Intn(10000)),
				LedgerMilestoneIndex: milestone.Index(rand.Intn(10000)),
				TreasuryOutput:       &utxo.TreasuryOutput{MilestoneID: iotago.MilestoneID{}, Amount: 13337},
			}

			originTimestamp := uint32(time.Now().Unix())

			// create generators and consumers
			sepIterFunc, sepGenRetriever := newSEPGenerator(150)
			sepConsumerFunc, sepsCollRetriever := newSEPCollector()

			outputIterFunc, outputGenRetriever := newOutputsGenerator(1000000)
			outputConsumerFunc, outputCollRetriever := newOutputCollector()

			msDiffIterFunc, msDiffGenRetriever := newMsDiffGenerator(50)
			msDiffConsumerFunc, msDiffCollRetriever := newMsDiffCollector()

			t := test{
				name:                          "full: 150 seps, 1 mil outputs, 50 ms diffs",
				snapshotFileName:              "full_snapshot.bin",
				originHeader:                  originHeader,
				originTimestamp:               originTimestamp,
				sepGenerator:                  sepIterFunc,
				sepGenRetriever:               sepGenRetriever,
				outputGenerator:               outputIterFunc,
				outputGenRetriever:            outputGenRetriever,
				msDiffGenerator:               msDiffIterFunc,
				msDiffGenRetriever:            msDiffGenRetriever,
				headerConsumer:                headerEqualFunc(t, originHeader),
				sepConsumer:                   sepConsumerFunc,
				sepConRetriever:               sepsCollRetriever,
				outputConsumer:                outputConsumerFunc,
				outputConRetriever:            outputCollRetriever,
				unspentTreasuryOutputConsumer: unspentTreasuryOutputEqualFunc(t, originHeader.TreasuryOutput),
				msDiffConsumer:                msDiffConsumerFunc,
				msDiffConRetriever:            msDiffCollRetriever,
			}
			return t
		}(),
		func() test {
			originHeader := &snapshot.FileHeader{
				Type:                 snapshot.Delta,
				Version:              snapshot.SupportedFormatVersion,
				NetworkID:            666666666,
				SEPMilestoneIndex:    milestone.Index(rand.Intn(10000)),
				LedgerMilestoneIndex: milestone.Index(rand.Intn(10000)),
			}

			originTimestamp := uint32(time.Now().Unix())

			// create generators and consumers
			sepIterFunc, sepGenRetriever := newSEPGenerator(150)
			sepConsumerFunc, sepsCollRetriever := newSEPCollector()

			msDiffIterFunc, msDiffGenRetriever := newMsDiffGenerator(50)
			msDiffConsumerFunc, msDiffCollRetriever := newMsDiffCollector()

			t := test{
				name:               "delta: 150 seps, 50 ms diffs",
				snapshotFileName:   "delta_snapshot.bin",
				originHeader:       originHeader,
				originTimestamp:    originTimestamp,
				sepGenerator:       sepIterFunc,
				sepGenRetriever:    sepGenRetriever,
				msDiffGenerator:    msDiffIterFunc,
				msDiffGenRetriever: msDiffGenRetriever,
				headerConsumer:     headerEqualFunc(t, originHeader),
				sepConsumer:        sepConsumerFunc,
				sepConRetriever:    sepsCollRetriever,
				msDiffConsumer:     msDiffConsumerFunc,
				msDiffConRetriever: msDiffCollRetriever,
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

			_, err = snapshot.StreamSnapshotDataTo(snapshotFileWrite, tt.originTimestamp, tt.originHeader, tt.sepGenerator, tt.outputGenerator, tt.msDiffGenerator)
			require.NoError(t, err)
			require.NoError(t, snapshotFileWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written (snapshot type: %d) snapshot file size: %s\n", tt.name, tt.originHeader.Type, humanize.Bytes(uint64(fileInfo.Size())))

			// read back written data and verify that it is equal
			snapshotFileRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			require.NoError(t, snapshot.StreamSnapshotDataFrom(snapshotFileRead, protoParas, tt.headerConsumer, tt.sepConsumer, tt.outputConsumer, tt.unspentTreasuryOutputConsumer, tt.msDiffConsumer))

			// verify that what has been written also has been read again
			require.EqualValues(t, tt.sepGenRetriever(), tt.sepConRetriever())
			if tt.originHeader.Type == snapshot.Full {
				EqualOutputs(t, tt.outputGenRetriever(), tt.outputConRetriever())
			}

			msDiffGen := tt.msDiffGenRetriever()
			msDiffCon := tt.msDiffConRetriever()
			for i := range msDiffGen {
				gen := msDiffGen[i]
				con := msDiffCon[i]
				require.EqualValues(t, gen.Milestone, con.Milestone)
				require.EqualValues(t, gen.SpentTreasuryOutput, con.SpentTreasuryOutput)
				EqualOutputs(t, gen.Created, con.Created)
				EqualSpents(t, gen.Consumed, con.Consumed)
			}
		})
	}

}

type sepRetrieverFunc func() iotago.BlockIDs

func newSEPGenerator(count int) (snapshot.SEPProducerFunc, sepRetrieverFunc) {
	var generatedSEPs iotago.BlockIDs
	return func() (iotago.BlockID, error) {
			if count == 0 {
				return iotago.EmptyBlockID(), snapshot.ErrNoMoreSEPToProduce
			}
			count--
			blockID := utils.RandBlockID()
			generatedSEPs = append(generatedSEPs, blockID)
			return blockID, nil
		}, func() iotago.BlockIDs {
			return generatedSEPs
		}
}

func newSEPCollector() (snapshot.SEPConsumerFunc, sepRetrieverFunc) {
	var generatedSEPs iotago.BlockIDs
	return func(sep iotago.BlockID) error {
			generatedSEPs = append(generatedSEPs, sep)
			return nil
		}, func() iotago.BlockIDs {
			return generatedSEPs
		}
}

type outputRetrieverFunc func() utxo.Outputs

func newOutputsGenerator(count int) (snapshot.OutputProducerFunc, outputRetrieverFunc) {
	var generatedOutputs utxo.Outputs
	return func() (*utxo.Output, error) {
			if count == 0 {
				return nil, nil
			}
			count--
			output := randLSTransactionUnspentOutputs()
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

func newMsDiffGenerator(count int) (snapshot.MilestoneDiffProducerFunc, msDiffRetrieverFunc) {
	var generateMsDiffs []*snapshot.MilestoneDiff
	pub, prv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}

	var mappingPubKey iotago.MilestonePublicKey
	copy(mappingPubKey[:], pub)
	pubKeys := []iotago.MilestonePublicKey{mappingPubKey}

	keyMapping := iotago.MilestonePublicKeyMapping{}
	keyMapping[mappingPubKey] = prv

	return func() (*snapshot.MilestoneDiff, error) {
			if count == 0 {
				return nil, nil
			}
			count--

			parents := iotago.BlockIDs{utils.RandBlockID()}
			milestonePayload := iotago.NewMilestone(rand.Uint32(), rand.Uint32(), protoParas.Version, utils.RandMilestoneID(), parents, utils.Rand32ByteHash(), utils.Rand32ByteHash())

			treasuryInput := &iotago.TreasuryInput{}
			copy(treasuryInput[:], utils.RandBytes(32))
			ed25519Addr := utils.RandAddress(iotago.AddressEd25519)
			migratedFundsEntry := &iotago.MigratedFundsEntry{Address: ed25519Addr, Deposit: rand.Uint64()}
			copy(migratedFundsEntry.TailTransactionHash[:], utils.RandBytes(49))
			receipt, err := iotago.NewReceiptBuilder(milestonePayload.Index).
				AddTreasuryTransaction(&iotago.TreasuryTransaction{
					Input:  treasuryInput,
					Output: &iotago.TreasuryOutput{Amount: rand.Uint64()},
				}).
				AddEntry(migratedFundsEntry).
				Build(protoParas)
			if err != nil {
				panic(err)
			}

			milestonePayload.Opts = iotago.MilestoneOpts{receipt}

			if err := milestonePayload.Sign(pubKeys, iotago.InMemoryEd25519MilestoneSigner(keyMapping)); err != nil {
				panic(err)
			}

			msDiff := &snapshot.MilestoneDiff{
				Milestone: milestonePayload,
			}

			createdCount := rand.Intn(500) + 1
			for i := 0; i < createdCount; i++ {
				msDiff.Created = append(msDiff.Created, randLSTransactionUnspentOutputs())
			}

			consumedCount := rand.Intn(500) + 1
			for i := 0; i < consumedCount; i++ {
				msDiff.Consumed = append(msDiff.Consumed, randLSTransactionSpents(milestone.Index(milestonePayload.Index)))
			}

			msDiff.SpentTreasuryOutput = &utxo.TreasuryOutput{
				MilestoneID: utils.Rand32ByteHash(),
				Amount:      uint64(rand.Intn(1000)),
				Spent:       true, // doesn't matter
			}

			generateMsDiffs = append(generateMsDiffs, msDiff)
			return msDiff, nil
		}, func() []*snapshot.MilestoneDiff {
			return generateMsDiffs
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

func headerEqualFunc(t *testing.T, originHeader *snapshot.FileHeader) snapshot.HeaderConsumerFunc {
	return func(readHeader *snapshot.ReadFileHeader) error {
		require.EqualValues(t, *originHeader, readHeader.FileHeader)
		return nil
	}
}

func unspentTreasuryOutputEqualFunc(t *testing.T, originUnspentTreasuryOutput *utxo.TreasuryOutput) snapshot.UnspentTreasuryOutputConsumerFunc {
	return func(readUnspentTreasuryOutput *utxo.TreasuryOutput) error {
		require.EqualValues(t, *originUnspentTreasuryOutput, *readUnspentTreasuryOutput)
		return nil
	}
}

func randLSTransactionUnspentOutputs() *utxo.Output {
	return utxo.CreateOutput(utils.RandOutputID(), utils.RandBlockID(), utils.RandMilestoneIndex(), rand.Uint32(), utils.RandOutput(utils.RandOutputType()))
}

func randLSTransactionSpents(msIndex milestone.Index) *utxo.Spent {
	return utxo.NewSpent(utxo.CreateOutput(utils.RandOutputID(), utils.RandBlockID(), utils.RandMilestoneIndex(), rand.Uint32(), utils.RandOutput(utils.RandOutputType())), utils.RandTransactionID(), msIndex, rand.Uint32())
}

func EqualOutput(t *testing.T, expected *utxo.Output, actual *utxo.Output) {
	require.Equal(t, expected.OutputID(), actual.OutputID())
	require.Equal(t, expected.BlockID(), actual.BlockID())
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
	require.Equal(t, expected.OutputType(), actual.OutputType())
	require.Equal(t, expected.Deposit(), actual.Deposit())
	require.EqualValues(t, expected.Output(), actual.Output())
}

func EqualSpent(t *testing.T, expected *utxo.Spent, actual *utxo.Spent) {
	require.Equal(t, expected.OutputID(), actual.OutputID())
	require.Equal(t, expected.TargetTransactionID(), actual.TargetTransactionID())
	require.Equal(t, expected.MilestoneIndex(), actual.MilestoneIndex())
	EqualOutput(t, expected.Output(), actual.Output())
}

func EqualOutputs(t *testing.T, expected utxo.Outputs, actual utxo.Outputs) {
	require.Equal(t, len(expected), len(actual))

	// Sort Outputs by output ID.
	sort.Slice(expected, func(i, j int) bool {
		iOutputID := expected[i].OutputID()
		jOutputID := expected[j].OutputID()
		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		iOutputID := actual[i].OutputID()
		jOutputID := actual[j].OutputID()
		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualOutput(t, expected[i], actual[i])
	}
}

func EqualSpents(t *testing.T, expected utxo.Spents, actual utxo.Spents) {
	require.Equal(t, len(expected), len(actual))

	// Sort Spents by output ID.
	sort.Slice(expected, func(i, j int) bool {
		iOutputID := expected[i].OutputID()
		jOutputID := expected[j].OutputID()
		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		iOutputID := actual[i].OutputID()
		jOutputID := actual[j].OutputID()
		return bytes.Compare(iOutputID[:], jOutputID[:]) == -1
	})

	for i := 0; i < len(expected); i++ {
		EqualSpent(t, expected[i], actual[i])
	}
}
