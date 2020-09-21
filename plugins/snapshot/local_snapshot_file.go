package snapshot

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/luca-moser/iota"
)

const (
	// The supported local snapshot file version.
	SupportedFormatVersion byte = 1
	// The length of a solid entry point hash.
	SolidEntryPointHashLength = iota.MessageHashLength
)

// TransactionOutputs are the unspent outputs under the same transaction hash within a local snapshot.
type TransactionOutputs struct {
	// The hash of the transaction.
	TransactionHash [iota.TransactionIDLength]byte `json:"transaction_hash"`
	// The unspent outputs belonging to this transaction.
	UnspentOutputs []*UnspentOutput `json:"unspent_outputs"`
}

func (s *TransactionOutputs) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	if _, err := b.Write(s.TransactionHash[:]); err != nil {
		return nil, err
	}

	// write count of outputs
	if err := binary.Write(&b, binary.LittleEndian, uint16(len(s.UnspentOutputs))); err != nil {
		return nil, err
	}

	for _, out := range s.UnspentOutputs {
		outData, err := out.MarshalBinary()
		if err != nil {
			return nil, err
		}

		if _, err := b.Write(outData); err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}

// UnspentOutput defines an unspent output within a local snapshot.
type UnspentOutput struct {
	// The index of the output.
	Index uint16 `json:"index"`
	// The underlying address to which this output deposits to.
	Address iota.Serializable `json:"address"`
	// The value of the deposit.
	Value uint64 `json:"value"`
}

func (s *UnspentOutput) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, s.Index); err != nil {
		return nil, err
	}
	addrData, err := s.Address.Serialize(iota.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}
	if _, err := b.Write(addrData); err != nil {
		return nil, err
	}
	if err := binary.Write(&b, binary.LittleEndian, s.Value); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// SEPIteratorFunc yields a solid entry point to be written to a local snapshot or nil if no more is available.
type SEPIteratorFunc func() *[SolidEntryPointHashLength]byte

// SEPConsumerFunc consumes the given solid entry point.
// A returned error signals to cancel further reading.
type SEPConsumerFunc func([SolidEntryPointHashLength]byte) error

// HeaderConsumerFunc consumes the local snapshot file header.
// A returned error signals to cancel further reading.
type HeaderConsumerFunc func(*FileHeader) error

// UTXOIteratorFunc yields a transaction and its outputs to be written to a local snapshot or nil if no more is available.
type UTXOIteratorFunc func() *TransactionOutputs

// UTXOConsumerFunc consumes the given transaction and its outputs.
// A returned error signals to cancel further reading.
type UTXOConsumerFunc func(*TransactionOutputs) error

// FileHeader is the file header of a local snapshot file.
type FileHeader struct {
	// Version denotes the version of this local snapshot.
	Version byte
	// The milestone index for which this local snapshot was taken.
	MilestoneIndex uint64
	// The hash of the milestone corresponding to the given milestone index.
	MilestoneHash [iota.MilestoneHashLength]byte
	// The time at which the local snapshot was taken.
	Timestamp uint64
	// The count of solid entry points.
	// This field is only available/used while reading a local snapshot.
	SEPCount uint64
	// The count of UTXOs.
	// This field is only available/used while reading a local snapshot.
	UTXOCount uint64
}

// StreamLocalSnapshotDataTo streams local snapshot data into the given io.WriteSeeker.
func StreamLocalSnapshotDataTo(writeSeeker io.WriteSeeker, header *FileHeader,
	sepIter SEPIteratorFunc, utxoIter UTXOIteratorFunc) error {

	// version, seps count, utxo count
	// timestamp, milestone index, milestone hash, seps, utxos
	var sepsCount, utxoCount uint64

	if _, err := writeSeeker.Write([]byte{header.Version}); err != nil {
		return err
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.Timestamp); err != nil {
		return err
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.MilestoneIndex); err != nil {
		return err
	}

	if _, err := writeSeeker.Write(header.MilestoneHash[:]); err != nil {
		return err
	}

	// write count and hash place holders
	if _, err := writeSeeker.Write(make([]byte, iota.UInt64ByteSize*2)); err != nil {
		return err
	}

	for sep := sepIter(); sep != nil; sep = sepIter() {
		_, err := writeSeeker.Write(sep[:])
		if err != nil {
			return err
		}
		sepsCount++
	}

	for utxo := utxoIter(); utxo != nil; utxo = utxoIter() {
		utxoData, err := utxo.MarshalBinary()
		if err != nil {
			return err
		}
		if _, err := writeSeeker.Write(utxoData); err != nil {
			return err
		}
		utxoCount++
	}

	// seek back to counts version+timestamp+msindex+mshash and write element counts
	if _, err := writeSeeker.Seek(iota.OneByte+iota.UInt64ByteSize+iota.UInt64ByteSize+iota.MilestoneHashLength, io.SeekStart); err != nil {
		return err
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return err
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, utxoCount); err != nil {
		return err
	}

	return nil
}

// StreamLocalSnapshotDataFrom consumes local snapshot data from the given reader.
func StreamLocalSnapshotDataFrom(reader io.Reader, headerConsumer HeaderConsumerFunc,
	sepConsumer SEPConsumerFunc, utxoConsumer UTXOConsumerFunc) error {
	header := &FileHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &header.Version); err != nil {
		return err
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.Timestamp); err != nil {
		return err
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.MilestoneIndex); err != nil {
		return err
	}

	if _, err := io.ReadFull(reader, header.MilestoneHash[:]); err != nil {
		return err
	}

	var sepsCount uint64
	if err := binary.Read(reader, binary.LittleEndian, &sepsCount); err != nil {
		return err
	}

	var utxoCount uint64
	if err := binary.Read(reader, binary.LittleEndian, &utxoCount); err != nil {
		return err
	}

	header.SEPCount = sepsCount
	header.UTXOCount = utxoCount
	if err := headerConsumer(header); err != nil {
		return err
	}

	for i := uint64(0); i < sepsCount; i++ {
		var sep [SolidEntryPointHashLength]byte
		if _, err := io.ReadFull(reader, sep[:]); err != nil {
			return err
		}

		// sep gets copied
		if err := sepConsumer(sep); err != nil {
			return err
		}
	}

	for i := uint64(0); i < utxoCount; i++ {
		utxo := &TransactionOutputs{}

		// read tx hash
		if _, err := io.ReadFull(reader, utxo.TransactionHash[:]); err != nil {
			return err
		}

		var outputsCount uint16
		if err := binary.Read(reader, binary.LittleEndian, &outputsCount); err != nil {
			return err
		}

		for j := uint16(0); j < outputsCount; j++ {
			output := &UnspentOutput{}

			if err := binary.Read(reader, binary.LittleEndian, &output.Index); err != nil {
				return err
			}

			// look ahead address type
			var addrTypeBuf [iota.SmallTypeDenotationByteSize]byte
			if _, err := io.ReadFull(reader, addrTypeBuf[:]); err != nil {
				return err
			}

			addrType := addrTypeBuf[0]
			addr, err := iota.AddressSelector(uint32(addrType))
			if err != nil {
				return err
			}

			var addrDataWithoutType []byte
			switch addr.(type) {
			case *iota.WOTSAddress:
				addrDataWithoutType = make([]byte, iota.WOTSAddressBytesLength)
			case *iota.Ed25519Address:
				addrDataWithoutType = make([]byte, iota.Ed25519AddressBytesLength)
			default:
				panic("unknown address type")
			}

			// read the rest of the address
			if _, err := io.ReadFull(reader, addrDataWithoutType); err != nil {
				return err
			}

			if _, err := addr.Deserialize(append(addrTypeBuf[:], addrDataWithoutType...), iota.DeSeriModePerformValidation); err != nil {
				return err
			}
			output.Address = addr

			if err := binary.Read(reader, binary.LittleEndian, &output.Value); err != nil {
				return err
			}

			utxo.UnspentOutputs = append(utxo.UnspentOutputs, output)
		}

		if err := utxoConsumer(utxo); err != nil {
			return err
		}
	}

	return nil
}
