package snapshot_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/blang/vfs/memfs"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/iotaledger/iota.go"
	"github.com/stretchr/testify/require"
)

type test struct {
	name             string
	snapshotFileName string
	originHeader     *snapshot.FileHeader
	sepGenerator     snapshot.SEPIteratorFunc
	sepGenRetriever  sepRetrieverFunc
	utxoGenerator    snapshot.UTXOIteratorFunc
	utxoGenRetriever utxoRetrieverFunc
	headerConsumer   snapshot.HeaderConsumerFunc
	sepConsumer      snapshot.SEPConsumerFunc
	sepConRetriever  sepRetrieverFunc
	utxoConsumer     snapshot.UTXOConsumerFunc
	utxoConRetriever utxoRetrieverFunc
}

func TestStreamLocalSnapshotDataToAndFrom(t *testing.T) {
	if testing.Short() {
		return
	}
	rand.Seed(346587549867)

	originHeader := &snapshot.FileHeader{
		Version: snapshot.SupportedFormatVersion, MilestoneIndex: uint64(rand.Intn(10000)),
		MilestoneHash: rand32ByteHash(), Timestamp: uint64(time.Now().Unix()),
	}

	testCases := []test{
		func() test {

			// create generators and consumers
			utxoIterFunc, utxosGenRetriever := newUTXOGenerator(1000000, 4)
			sepIterFunc, sepGenRetriever := newSEPGenerator(150)
			sepConsumerfunc, sepsCollRetriever := newSEPCollector()
			utxoConsumerFunc, utxoCollRetriever := newUTXOCollector()

			t := test{
				name:             "150 seps, 1 mil txs, uncompressed",
				snapshotFileName: "uncompressed_snapshot.bin",
				originHeader:     originHeader,
				sepGenerator:     sepIterFunc,
				sepGenRetriever:  sepGenRetriever,
				utxoGenerator:    utxoIterFunc,
				utxoGenRetriever: utxosGenRetriever,
				headerConsumer:   headerEqualFunc(t, originHeader),
				sepConsumer:      sepConsumerfunc,
				sepConRetriever:  sepsCollRetriever,
				utxoConsumer:     utxoConsumerFunc,
				utxoConRetriever: utxoCollRetriever,
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

			require.NoError(t, snapshot.StreamLocalSnapshotDataTo(snapshotFileWrite, tt.originHeader, tt.sepGenerator, tt.utxoGenerator))
			require.NoError(t, snapshotFileWrite.Close())

			fileInfo, err := fs.Stat(filePath)
			require.NoError(t, err)
			fmt.Printf("%s: written local snapshot file size: %d MB\n", tt.name, fileInfo.Size()/1024/1024)

			// read back written data and verify that it is equal
			snapshotFileRead, err := fs.OpenFile(filePath, os.O_RDONLY, 0666)
			require.NoError(t, err)

			require.NoError(t, snapshot.StreamLocalSnapshotDataFrom(snapshotFileRead, tt.headerConsumer, tt.sepConsumer, tt.utxoConsumer))

			utxoGenerated, _ := tt.utxoGenRetriever()
			utxoConsumed, _ := tt.utxoConRetriever()
			require.EqualValues(t, utxoGenerated, utxoConsumed)
			require.EqualValues(t, tt.sepGenRetriever(), tt.sepConRetriever())
		})
	}

}

type sepRetrieverFunc func() [][snapshot.SolidEntryPointHashLength]byte

func newSEPGenerator(count int) (snapshot.SEPIteratorFunc, sepRetrieverFunc) {
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

type utxoRetrieverFunc func() ([]snapshot.TransactionOutputs, uint64)

func newUTXOGenerator(count int, maxRandOutputsPerTx int) (snapshot.UTXOIteratorFunc, utxoRetrieverFunc) {
	var generatedUTXOs []snapshot.TransactionOutputs
	var outputsTotal uint64
	return func() *snapshot.TransactionOutputs {
			if count == 0 {
				return nil
			}
			count--
			outputsCount := rand.Intn(maxRandOutputsPerTx) + 1
			tx := randLSTransactionUnspentOutputs(outputsCount)
			generatedUTXOs = append(generatedUTXOs, *tx)
			outputsTotal += uint64(len(tx.UnspentOutputs))
			return tx
		}, func() ([]snapshot.TransactionOutputs, uint64) {
			return generatedUTXOs, outputsTotal
		}
}

func newUTXOCollector() (snapshot.UTXOConsumerFunc, utxoRetrieverFunc) {
	var generatedUTXOs []snapshot.TransactionOutputs
	return func(utxo *snapshot.TransactionOutputs) error {
			generatedUTXOs = append(generatedUTXOs, *utxo)
			return nil
		}, func() ([]snapshot.TransactionOutputs, uint64) {
			return generatedUTXOs, 0
		}
}

func headerEqualFunc(t *testing.T, originHeader *snapshot.FileHeader) snapshot.HeaderConsumerFunc {
	return func(readHeader *snapshot.FileHeader) error {
		readHeader.UTXOCount = 0
		readHeader.SEPCount = 0
		require.Equal(t, originHeader, readHeader)
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

func randLSTransactionUnspentOutputs(outputsCount int) *snapshot.TransactionOutputs {
	return &snapshot.TransactionOutputs{
		TransactionHash: rand32ByteHash(),
		UnspentOutputs: func() []*snapshot.UnspentOutput {
			outputs := make([]*snapshot.UnspentOutput, outputsCount)
			for i := 0; i < outputsCount; i++ {
				addr, _ := randEd25519Addr()
				outputs[i] = &snapshot.UnspentOutput{
					Index:   uint16(i),
					Address: addr,
					Value:   uint64(rand.Intn(1000000) + 1),
				}
			}
			return outputs
		}(),
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
