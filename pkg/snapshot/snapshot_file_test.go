package snapshot_test

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/blang/vfs/memfs"
	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

type test struct {
	name                          string
	snapshotFileName              string
	originHeader                  *snapshot.FileHeader
	originTimestamp               uint64
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

			originTimestamp := uint64(time.Now().Unix())

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

			originTimestamp := uint64(time.Now().Unix())

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

			require.NoError(t, snapshot.StreamSnapshotDataFrom(snapshotFileRead, tt.headerConsumer, tt.sepConsumer, tt.outputConsumer, tt.unspentTreasuryOutputConsumer, tt.msDiffConsumer))

			// verify that what has been written also has been read again
			require.EqualValues(t, tt.sepGenRetriever(), tt.sepConRetriever())
			if tt.originHeader.Type == snapshot.Full {
				require.EqualValues(t, tt.outputGenRetriever(), tt.outputConRetriever())
			}
			require.EqualValues(t, tt.msDiffGenRetriever(), tt.msDiffConRetriever())
		})
	}

}

type sepRetrieverFunc func() hornet.MessageIDs

func newSEPGenerator(count int) (snapshot.SEPProducerFunc, sepRetrieverFunc) {
	var generatedSEPs hornet.MessageIDs
	return func() (hornet.MessageID, error) {
			if count == 0 {
				return nil, nil
			}
			count--
			msgID := randMessageID()
			generatedSEPs = append(generatedSEPs, msgID)
			return msgID, nil
		}, func() hornet.MessageIDs {
			return generatedSEPs
		}
}

func newSEPCollector() (snapshot.SEPConsumerFunc, sepRetrieverFunc) {
	var generatedSEPs hornet.MessageIDs
	return func(sep hornet.MessageID) error {
			generatedSEPs = append(generatedSEPs, sep)
			return nil
		}, func() hornet.MessageIDs {
			return generatedSEPs
		}
}

type outputRetrieverFunc func() []snapshot.Output

func newOutputsGenerator(count int) (snapshot.OutputProducerFunc, outputRetrieverFunc) {
	var generatedOutputs []snapshot.Output
	return func() (*snapshot.Output, error) {
			if count == 0 {
				return nil, nil
			}
			count--
			output := randLSTransactionUnspentOutputs()
			generatedOutputs = append(generatedOutputs, *output)
			return output, nil
		}, func() []snapshot.Output {
			return generatedOutputs
		}
}

func newOutputCollector() (snapshot.OutputConsumerFunc, outputRetrieverFunc) {
	var generatedOutputs []snapshot.Output
	return func(utxo *snapshot.Output) error {
			generatedOutputs = append(generatedOutputs, *utxo)
			return nil
		}, func() []snapshot.Output {
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

			parents := iotago.MilestoneParentMessageIDs{rand32ByteHash()}
			ms, err := iotago.NewMilestone(rand.Uint32(), rand.Uint64(), parents, rand32ByteHash(), pubKeys)
			if err != nil {
				panic(err)
			}

			treasuryInput := &iotago.TreasuryInput{}
			copy(treasuryInput[:], randBytes(32))
			ed25519Addr, _ := randEd25519Addr()
			migratedFundsEntry := &iotago.MigratedFundsEntry{Address: ed25519Addr, Deposit: rand.Uint64()}
			copy(migratedFundsEntry.TailTransactionHash[:], randBytes(49))
			receipt, err := iotago.NewReceiptBuilder(ms.Index).
				AddTreasuryTransaction(&iotago.TreasuryTransaction{
					Input:  treasuryInput,
					Output: &iotago.TreasuryOutput{Amount: rand.Uint64()},
				}).
				AddEntry(migratedFundsEntry).
				Build()
			if err != nil {
				panic(err)
			}

			ms.Receipt = receipt

			if err := ms.Sign(iotago.InMemoryEd25519MilestoneSigner(keyMapping)); err != nil {
				panic(err)
			}

			msDiff := &snapshot.MilestoneDiff{
				Milestone: ms,
			}

			createdCount := rand.Intn(500) + 1
			for i := 0; i < createdCount; i++ {
				msDiff.Created = append(msDiff.Created, randLSTransactionUnspentOutputs())
			}

			consumedCount := rand.Intn(500) + 1
			for i := 0; i < consumedCount; i++ {
				msDiff.Consumed = append(msDiff.Consumed, randLSTransactionSpents())
			}

			msDiff.SpentTreasuryOutput = &utxo.TreasuryOutput{
				MilestoneID: rand32ByteHash(),
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

func rand32ByteHash() [iotago.TransactionIDLength]byte {
	var h [iotago.TransactionIDLength]byte
	b := randBytes(32)
	copy(h[:], b)
	return h
}

func randLSTransactionUnspentOutputs() *snapshot.Output {
	addr, _ := randEd25519Addr()

	var outputID [utxo.OutputIDLength]byte
	transactionID := rand32ByteHash()
	copy(outputID[:], transactionID[:])
	binary.LittleEndian.PutUint16(outputID[iotago.TransactionIDLength:], uint16(rand.Intn(100)))

	return &snapshot.Output{
		MessageID:  randMessageID().ToArray(),
		OutputID:   outputID,
		OutputType: byte(rand.Intn(256)),
		Address:    addr,
		Amount:     uint64(rand.Intn(1000000) + 1),
	}
}

func randLSTransactionSpents() *snapshot.Spent {
	addr, _ := randEd25519Addr()

	var outputID [utxo.OutputIDLength]byte
	transactionID := rand32ByteHash()
	copy(outputID[:], transactionID[:])
	binary.LittleEndian.PutUint16(outputID[iotago.TransactionIDLength:], uint16(rand.Intn(100)))

	output := &snapshot.Output{
		MessageID:  randMessageID().ToArray(),
		OutputID:   outputID,
		OutputType: byte(rand.Intn(256)),
		Address:    addr,
		Amount:     uint64(rand.Intn(1000000) + 1),
	}

	return &snapshot.Spent{Output: *output, TargetTransactionID: rand32ByteHash()}
}

//nolint:unparam
func randEd25519Addr() (*iotago.Ed25519Address, []byte) {
	// type
	edAddr := &iotago.Ed25519Address{}
	addr := randBytes(iotago.Ed25519AddressBytesLength)
	copy(edAddr[:], addr)
	// serialized
	var b [iotago.Ed25519AddressSerializedBytesSize]byte
	b[0] = iotago.AddressEd25519
	copy(b[iotago.SmallTypeDenotationByteSize:], addr)
	return edAddr, b[:]
}
