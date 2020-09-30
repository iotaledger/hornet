package snapshot_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/blang/vfs/memfs"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/iotaledger/iota.go"
	"github.com/stretchr/testify/require"
)

type test struct {
	name               string
	snapshotFileName   string
	originHeader       *snapshot.FileHeader
	originTimestamp    uint64
	sepGenerator       snapshot.SEPProducerFunc
	sepGenRetriever    sepRetrieverFunc
	outputGenerator    snapshot.OutputProducerFunc
	outputGenRetriever outputRetrieverFunc
	msDiffGenerator    snapshot.MilestoneDiffProducerFunc
	msDiffGenRetriever msDiffRetrieverFunc
	headerConsumer     snapshot.HeaderConsumerFunc
	sepConsumer        snapshot.SEPConsumerFunc
	sepConRetriever    sepRetrieverFunc
	outputConsumer     snapshot.OutputConsumerFunc
	outputConRetriever outputRetrieverFunc
	msDiffConsumer     snapshot.MilestoneDiffConsumerFunc
	msDiffConRetriever msDiffRetrieverFunc
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
				SEPMilestoneIndex:    milestone.Index(rand.Intn(10000)),
				SEPMilestoneHash:     rand32ByteHash(),
				LedgerMilestoneIndex: milestone.Index(rand.Intn(10000)),
				LedgerMilestoneHash:  rand32ByteHash(),
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
				name:               "full: 150 seps, 1 mil outputs, 50 ms diffs",
				snapshotFileName:   "full_snapshot.bin",
				originHeader:       originHeader,
				originTimestamp:    originTimestamp,
				sepGenerator:       sepIterFunc,
				sepGenRetriever:    sepGenRetriever,
				outputGenerator:    outputIterFunc,
				outputGenRetriever: outputGenRetriever,
				msDiffGenerator:    msDiffIterFunc,
				msDiffGenRetriever: msDiffGenRetriever,
				headerConsumer:     headerEqualFunc(t, originHeader),
				sepConsumer:        sepConsumerFunc,
				sepConRetriever:    sepsCollRetriever,
				outputConsumer:     outputConsumerFunc,
				outputConRetriever: outputCollRetriever,
				msDiffConsumer:     msDiffConsumerFunc,
				msDiffConRetriever: msDiffCollRetriever,
			}
			return t
		}(),
		func() test {

			originHeader := &snapshot.FileHeader{
				Type:                 snapshot.Delta,
				Version:              snapshot.SupportedFormatVersion,
				SEPMilestoneIndex:    milestone.Index(rand.Intn(10000)),
				SEPMilestoneHash:     rand32ByteHash(),
				LedgerMilestoneIndex: milestone.Index(rand.Intn(10000)),
				LedgerMilestoneHash:  rand32ByteHash(),
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

			require.NoError(t, snapshot.StreamLocalSnapshotDataTo(snapshotFileWrite, tt.originTimestamp, tt.originHeader, tt.sepGenerator, tt.outputGenerator, tt.msDiffGenerator))
			require.NoError(t, snapshotFileWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written (snapshot type: %d) local snapshot file size: %d MB\n", tt.name, tt.originHeader.Type, fileInfo.Size()/1024/1024)

			// read back written data and verify that it is equal
			snapshotFileRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			require.NoError(t, snapshot.StreamLocalSnapshotDataFrom(snapshotFileRead, tt.headerConsumer, tt.sepConsumer, tt.outputConsumer, tt.msDiffConsumer))

			// verify that what has been written also has been read again
			require.EqualValues(t, tt.sepGenRetriever(), tt.sepConRetriever())
			if tt.originHeader.Type == snapshot.Full {
				require.EqualValues(t, tt.outputGenRetriever(), tt.outputConRetriever())
			}
			require.EqualValues(t, tt.msDiffGenRetriever(), tt.msDiffConRetriever())
		})
	}

}

type sepRetrieverFunc func() [][snapshot.SolidEntryPointHashLength]byte

func newSEPGenerator(count int) (snapshot.SEPProducerFunc, sepRetrieverFunc) {
	var generatedSEPs [][snapshot.SolidEntryPointHashLength]byte
	return func() *[snapshot.SolidEntryPointHashLength]byte {
			if count == 0 {
				return nil
			}
			count--
			x := rand32ByteHash()
			generatedSEPs = append(generatedSEPs, x)
			return &x
		}, func() [][32]byte {
			return generatedSEPs
		}
}

func newSEPCollector() (snapshot.SEPConsumerFunc, sepRetrieverFunc) {
	var generatedSEPs [][snapshot.SolidEntryPointHashLength]byte
	return func(sep [snapshot.SolidEntryPointHashLength]byte) error {
			generatedSEPs = append(generatedSEPs, sep)
			return nil
		}, func() [][32]byte {
			return generatedSEPs
		}
}

type outputRetrieverFunc func() []snapshot.Output

func newOutputsGenerator(count int) (snapshot.OutputProducerFunc, outputRetrieverFunc) {
	var generatedOutputs []snapshot.Output
	return func() *snapshot.Output {
			if count == 0 {
				return nil
			}
			count--
			output := randLSTransactionUnspentOutputs()
			generatedOutputs = append(generatedOutputs, *output)
			return output
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
	return func() *snapshot.MilestoneDiff {
			if count == 0 {
				return nil
			}
			count--

			msDiff := &snapshot.MilestoneDiff{
				MilestoneIndex: milestone.Index(rand.Int63()),
			}

			createdCount := rand.Intn(500) + 1
			for i := 0; i < createdCount; i++ {
				msDiff.Created = append(msDiff.Created, randLSTransactionUnspentOutputs())
			}

			consumedCount := rand.Intn(500) + 1
			for i := 0; i < consumedCount; i++ {
				msDiff.Consumed = append(msDiff.Consumed, randLSTransactionUnspentOutputs())
			}

			generateMsDiffs = append(generateMsDiffs, msDiff)
			return msDiff
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

func randBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func rand32ByteHash() [iota.TransactionIDLength]byte {
	var h [iota.TransactionIDLength]byte
	b := randBytes(32)
	copy(h[:], b)
	return h
}

func randLSTransactionUnspentOutputs() *snapshot.Output {
	addr, _ := randEd25519Addr()
	return &snapshot.Output{
		TransactionID: rand32ByteHash(),
		OutputIndex:   uint16(rand.Intn(100)),
		Address:       addr,
		Amount:        uint64(rand.Intn(1000000) + 1),
	}
}

func randEd25519Addr() (*iota.Ed25519Address, []byte) {
	// type
	edAddr := &iota.Ed25519Address{}
	addr := randBytes(iota.Ed25519AddressBytesLength)
	copy(edAddr[:], addr)
	// serialized
	var b [iota.Ed25519AddressSerializedBytesSize]byte
	b[0] = iota.AddressEd25519
	copy(b[iota.SmallTypeDenotationByteSize:], addr)
	return edAddr, b[:]
}
