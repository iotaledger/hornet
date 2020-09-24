package snapshot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	iotago "github.com/iotaledger/iota.go"
)

const (
	// The supported local snapshot file version.
	SupportedFormatVersion byte = 1
	// The length of a solid entry point hash.
	SolidEntryPointHashLength = iotago.MessageHashLength

	// The offset of counters within a local snapshot file:
	// version+type+timestamp+sep-ms-index+sep-ms-hash+ledger-ms-index+ledger-ms-hash
	countersOffset = iotago.OneByte + iotago.OneByte + iotago.UInt64ByteSize +
		iotago.UInt64ByteSize + iotago.MilestonePayloadHashLength +
		iotago.UInt64ByteSize + iotago.MilestonePayloadHashLength
)

var (
	// Returned when a wrong snapshot type is being read.
	ErrWrongSnapshotType = errors.New("wrong snapshot type")
)

// Type defines the type of the local snapshot.
type Type byte

const (
	// Full is a local snapshot which contains the full ledger entry for a given milestone
	// plus the milestone diffs which subtracted to the ledger milestone reduce to the snapshot milestone ledger.
	Full Type = iota
	// Delta is a local snapshot which contains solely diffs of milestones newer than a certain ledger milestone.
	Delta
)

// Output defines an output within a local snapshot.
type Output struct {
	TransactionHash [iotago.TransactionIDLength]byte `json:"transaction_hash"`
	// The index of the output.
	Index uint16 `json:"index"`
	// The underlying address to which this output deposits to.
	Address iotago.Serializable `json:"address"`
	// The value of the deposit.
	Value uint64 `json:"value"`
}

func (s *Output) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	if _, err := b.Write(s.TransactionHash[:]); err != nil {
		return nil, fmt.Errorf("unable to write transaction hash for ls-output: %w", err)
	}
	if err := binary.Write(&b, binary.LittleEndian, s.Index); err != nil {
		return nil, fmt.Errorf("unable to write index for ls-output: %w", err)
	}
	addrData, err := s.Address.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize address for ls-output: %w", err)
	}
	if _, err := b.Write(addrData); err != nil {
		return nil, fmt.Errorf("unable to write address for ls-output: %w", err)
	}
	if err := binary.Write(&b, binary.LittleEndian, s.Value); err != nil {
		return nil, fmt.Errorf("unable to write value for ls-output: %w", err)
	}
	return b.Bytes(), nil
}

// MilestoneDiff represents the outputs which were created and consumed for the given milestone.
type MilestoneDiff struct {
	MilestoneIndex uint64    `json:"milestone_index"`
	Created        []*Output `json:"created"`
	Consumed       []*Output `json:"consumed"`
}

func (md *MilestoneDiff) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, md.MilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to write milestone index for ls-milestone-diff %d: %w", md.MilestoneIndex, err)
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Created))); err != nil {
		return nil, fmt.Errorf("unable to write created outputs array length for ls-milestone-diff %d: %w", md.MilestoneIndex, err)
	}

	for x, output := range md.Created {
		outputBytes, err := output.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize output %d for ls-milestone-diff %d: %w", x, md.MilestoneIndex, err)
		}
		if _, err := b.Write(outputBytes); err != nil {
			return nil, fmt.Errorf("unable to write output %d for ls-milestone-diff %d: %w", x, md.MilestoneIndex, err)
		}
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Consumed))); err != nil {
		return nil, fmt.Errorf("unable to write consumed outputs array length for ls-milestone-diff %d: %w", md.MilestoneIndex, err)
	}

	for x, output := range md.Consumed {
		outputBytes, err := output.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize output %d for ls-milestone-diff %d: %w", x, md.MilestoneIndex, err)
		}
		if _, err := b.Write(outputBytes); err != nil {
			return nil, fmt.Errorf("unable to write output %d for ls-milestone-diff %d: %w", x, md.MilestoneIndex, err)
		}
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
type HeaderConsumerFunc func(*ReadFileHeader) error

// OutputIteratorFunc yields an output to be written to a local snapshot or nil if no more is available.
type OutputIteratorFunc func() *Output

// OutputConsumerFunc consumes the given output.
// A returned error signals to cancel further reading.
type OutputConsumerFunc func(output *Output) error

// MilestoneDiffIteratorFunc yields a milestone diff to be written to a local snapshot or nil if no more is available.
type MilestoneDiffIteratorFunc func() *MilestoneDiff

// MilestoneDiffConsumerFunc consumes the given MilestoneDiff.
// A returned error signals to cancel further reading.
type MilestoneDiffConsumerFunc func(milestoneDiff *MilestoneDiff) error

// FileHeader is the file header of a local snapshot file.
type FileHeader struct {
	// Version denotes the version of this local snapshot.
	Version byte
	// Type denotes the type of this local snapshot.
	Type Type
	// The milestone index of the SEPs for which this local snapshot was taken.
	SEPMilestoneIndex uint64
	// The hash of the milestone of the SEPs.
	SEPMilestoneHash [iotago.MilestonePayloadHashLength]byte
	// The milestone index of the ledger data within the local snapshot.
	LedgerMilestoneIndex uint64
	// The hash of the ledger milestone.
	LedgerMilestoneHash [iotago.MilestonePayloadHashLength]byte
}

// ReadFileHeader is a FileHeader but with additional content read from the local snapshot.
type ReadFileHeader struct {
	FileHeader
	// The time at which the local snapshot was taken.
	Timestamp uint64
	// The count of solid entry points.
	SEPCount uint64
	// The count of UTXOs.
	UTXOCount uint64
	// The count of milestone diffs.
	MilestoneDiffCount uint64
}

// StreamFullLocalSnapshotDataTo streams a full local snapshot data into the given io.WriteSeeker.
func StreamFullLocalSnapshotDataTo(writeSeeker io.WriteSeeker, timestamp uint64, header *FileHeader,
	sepIter SEPIteratorFunc, outputIter OutputIteratorFunc, msDiffIter MilestoneDiffIteratorFunc) error {

	var sepsCount, utxoCount, msDiffCount uint64

	// write LS file version and type
	if _, err := writeSeeker.Write([]byte{header.Version, byte(Full)}); err != nil {
		return fmt.Errorf("unable to write LS version and type: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, timestamp); err != nil {
		return fmt.Errorf("unable to write LS timestamp: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.SEPMilestoneIndex); err != nil {
		return fmt.Errorf("unable to write LS SEPs milestone index: %w", err)
	}

	if _, err := writeSeeker.Write(header.SEPMilestoneHash[:]); err != nil {
		return fmt.Errorf("unable to write LS SEPs milestone hash: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.LedgerMilestoneIndex); err != nil {
		return fmt.Errorf("unable to write LS ledger milestone index: %w", err)
	}

	if _, err := writeSeeker.Write(header.LedgerMilestoneHash[:]); err != nil {
		return fmt.Errorf("unable to write LS ledger milestone hash: %w", err)
	}

	// write count and hash place holders
	if _, err := writeSeeker.Write(make([]byte, iotago.UInt64ByteSize*3)); err != nil {
		return fmt.Errorf("unable to write LS SEPs/Outputs/Diffs placeholders: %w", err)
	}

	for sep := sepIter(); sep != nil; sep = sepIter() {
		sepsCount++
		if _, err := writeSeeker.Write(sep[:]); err != nil {
			return fmt.Errorf("unable to write LS SEP #%d: %w", sepsCount, err)
		}
	}

	for output := outputIter(); output != nil; output = outputIter() {
		utxoCount++
		outputBytes, err := output.MarshalBinary()
		if err != nil {
			return fmt.Errorf("unable to serialize LS output #%d: %w", utxoCount, err)
		}
		if _, err := writeSeeker.Write(outputBytes); err != nil {
			return fmt.Errorf("unable to write LS output #%d: %w", utxoCount, err)
		}
	}

	for msDiff := msDiffIter(); msDiff != nil; msDiff = msDiffIter() {
		msDiffCount++
		msDiffBytes, err := msDiff.MarshalBinary()
		if err != nil {
			return fmt.Errorf("unable to serialize LS milestone diff #%d: %w", msDiffCount, err)
		}
		if _, err := writeSeeker.Write(msDiffBytes); err != nil {
			return fmt.Errorf("unable to write LS milestone diff #%d: %w", msDiffCount, err)
		}
	}

	if _, err := writeSeeker.Seek(countersOffset, io.SeekStart); err != nil {
		return fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return fmt.Errorf("unable to write to LS SEPs count: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, utxoCount); err != nil {
		return fmt.Errorf("unable to write to LS outputs count: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return fmt.Errorf("unable to write to LS ms-diffs count: %w", err)
	}

	return nil
}

// StreamFullLocalSnapshotDataFrom consumes a full local snapshot from the given reader.
func StreamFullLocalSnapshotDataFrom(reader io.Reader, headerConsumer HeaderConsumerFunc,
	sepConsumer SEPConsumerFunc, outputConsumer OutputConsumerFunc, msDiffConsumer MilestoneDiffConsumerFunc) error {
	header := &ReadFileHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &header.Version); err != nil {
		return fmt.Errorf("unable to read LS version: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.Type); err != nil {
		return fmt.Errorf("unable to read LS type: %w", err)
	}

	if header.Type != Full {
		return fmt.Errorf("%w: expected to read a full local snapshot but got: %d", ErrWrongSnapshotType, header.Type)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.Timestamp); err != nil {
		return fmt.Errorf("unable to read LS timestamp: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.SEPMilestoneIndex); err != nil {
		return fmt.Errorf("unable to read LS SEPs milestone index: %w", err)
	}

	if _, err := io.ReadFull(reader, header.SEPMilestoneHash[:]); err != nil {
		return fmt.Errorf("unable to read LS SEPs milestone hash: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.LedgerMilestoneIndex); err != nil {
		return fmt.Errorf("unable to read LS ledger milestone index: %w", err)
	}

	if _, err := io.ReadFull(reader, header.LedgerMilestoneHash[:]); err != nil {
		return fmt.Errorf("unable to read LS ledger milestone hash: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.SEPCount); err != nil {
		return fmt.Errorf("unable to read LS SEPs count: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.UTXOCount); err != nil {
		return fmt.Errorf("unable to read LS outputs count: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &header.MilestoneDiffCount); err != nil {
		return fmt.Errorf("unable to read LS ms-diff count: %w", err)
	}

	if err := headerConsumer(header); err != nil {
		return err
	}

	for i := uint64(0); i < header.SEPCount; i++ {
		var sep [SolidEntryPointHashLength]byte
		if _, err := io.ReadFull(reader, sep[:]); err != nil {
			return fmt.Errorf("unable to read LS SEP at pos %d: %w", i, err)
		}
		if err := sepConsumer(sep); err != nil {
			return err
		}
	}

	for i := uint64(0); i < header.UTXOCount; i++ {
		output, err := readOutput(reader)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}

		if err := outputConsumer(output); err != nil {
			return err
		}
	}

	for i := uint64(0); i < header.MilestoneDiffCount; i++ {
		msDiff, err := readMilestoneDiff(reader)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}
		if err := msDiffConsumer(msDiff); err != nil {
			return err
		}
	}

	return nil
}

// reads a MilestoneDiff from the given reader.
func readMilestoneDiff(reader io.Reader) (*MilestoneDiff, error) {
	msDiff := &MilestoneDiff{}

	if err := binary.Read(reader, binary.LittleEndian, &msDiff.MilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff ms-index: %w", err)
	}

	var createdCount, consumedCount uint64
	if err := binary.Read(reader, binary.LittleEndian, &createdCount); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff created count: %w", err)
	}

	msDiff.Created = make([]*Output, createdCount)
	for i := uint64(0); i < createdCount; i++ {
		diffCreatedOutput, err := readOutput(reader)
		if err != nil {
			return nil, fmt.Errorf("(ms-diff created-output) at pos %d: %w", i, err)
		}
		msDiff.Created[i] = diffCreatedOutput
	}

	if err := binary.Read(reader, binary.LittleEndian, &consumedCount); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff consumed count: %w", err)
	}

	msDiff.Consumed = make([]*Output, consumedCount)
	for i := uint64(0); i < consumedCount; i++ {
		diffConsumedOutput, err := readOutput(reader)
		if err != nil {
			return nil, fmt.Errorf("(ms-diff consumed-output) at pos %d: %w", i, err)
		}
		msDiff.Consumed[i] = diffConsumedOutput
	}

	return msDiff, nil
}

// reads an Output from the given reader.
func readOutput(reader io.Reader) (*Output, error) {
	output := &Output{}
	if _, err := io.ReadFull(reader, output.TransactionHash[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS output tx hash: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &output.Index); err != nil {
		return nil, fmt.Errorf("unable to read LS output index: %w", err)
	}

	// look ahead address type
	var addrTypeBuf [iotago.SmallTypeDenotationByteSize]byte
	if _, err := io.ReadFull(reader, addrTypeBuf[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS output address type byte: %w", err)
	}

	addrType := addrTypeBuf[0]
	addr, err := iotago.AddressSelector(uint32(addrType))
	if err != nil {
		return nil, fmt.Errorf("unable to determine address type of LS output: %w", err)
	}

	var addrDataWithoutType []byte
	switch addr.(type) {
	case *iotago.WOTSAddress:
		addrDataWithoutType = make([]byte, iotago.WOTSAddressBytesLength)
	case *iotago.Ed25519Address:
		addrDataWithoutType = make([]byte, iotago.Ed25519AddressBytesLength)
	default:
		panic("unknown address type")
	}

	// read the rest of the address
	if _, err := io.ReadFull(reader, addrDataWithoutType); err != nil {
		return nil, fmt.Errorf("unable to read LS output address: %w", err)
	}

	if _, err := addr.Deserialize(append(addrTypeBuf[:], addrDataWithoutType...), iotago.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("invalid LS output address: %w", err)
	}
	output.Address = addr

	if err := binary.Read(reader, binary.LittleEndian, &output.Value); err != nil {
		return nil, fmt.Errorf("unable to read LS output value: %w", err)
	}

	return output, nil
}
