package snapshot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/byteutils"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	// The supported snapshot file version.
	SupportedFormatVersion byte = 1
	// The length of a solid entry point hash.
	SolidEntryPointHashLength = iotago.MessageIDLength

	// The offset of counters within a snapshot file:
	// version + type + timestamp + network-id + sep-ms-index + ledger-ms-index
	countersOffset = iotago.OneByte + iotago.OneByte + iotago.UInt64ByteSize + iotago.UInt64ByteSize +
		iotago.UInt32ByteSize + iotago.UInt32ByteSize
)

var (
	// Returned when an output producer has not been provided.
	ErrOutputProducerNotProvided = errors.New("output producer is not provided")
	// Returned when an output consumer has not been provided.
	ErrOutputConsumerNotProvided = errors.New("output consumer is not provided")
)

// Type defines the type of the snapshot.
type Type byte

const (
	// Full is a snapshot which contains the full ledger entry for a given milestone
	// plus the milestone diffs which subtracted to the ledger milestone reduce to the snapshot milestone ledger.
	Full Type = iota
	// Delta is a snapshot which contains solely diffs of milestones newer than a certain ledger milestone
	// instead of the complete ledger state of a given milestone.
	Delta
)

// Output defines an output within a snapshot.
type Output struct {
	// The message ID of the message that contained the transaction where this output was created.
	MessageID [iotago.MessageIDLength]byte `json:"message_id"`
	// The transaction ID and the index of the output.
	OutputID [iotago.TransactionIDLength + 2]byte `json:"output_id"`
	// The underlying address to which this output deposits to.
	Address iotago.Serializable `json:"address"`
	// The amount of the deposit.
	Amount uint64 `json:"amount"`
}

func (s *Output) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	if _, err := b.Write(s.MessageID[:]); err != nil {
		return nil, fmt.Errorf("unable to write message ID for ls-output: %w", err)
	}
	if _, err := b.Write(s.OutputID[:]); err != nil {
		return nil, fmt.Errorf("unable to write output ID for ls-output: %w", err)
	}
	addrData, err := s.Address.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize address for ls-output: %w", err)
	}
	if _, err := b.Write(addrData); err != nil {
		return nil, fmt.Errorf("unable to write address for ls-output: %w", err)
	}
	if err := binary.Write(&b, binary.LittleEndian, s.Amount); err != nil {
		return nil, fmt.Errorf("unable to write value for ls-output: %w", err)
	}
	return b.Bytes(), nil
}

// Spent defines a spent within a snapshot.
type Spent struct {
	Output
	// The transaction ID the funds were spent with.
	TargetTransactionID [iotago.TransactionIDLength]byte `json:"target_transaction_id"`
}

func (s *Spent) MarshalBinary() ([]byte, error) {
	bytes, err := s.Output.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return byteutils.ConcatBytes(bytes, s.TargetTransactionID[:]), nil
}

// MilestoneDiff represents the outputs which were created and consumed for the given milestone.
type MilestoneDiff struct {
	// The index of the milestone for which the diff applies.
	MilestoneIndex milestone.Index `json:"milestone_index"`
	// The created outputs with this milestone.
	Created []*Output `json:"created"`
	// The consumed spents with this milestone.
	Consumed []*Spent `json:"consumed"`
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

	for x, spent := range md.Consumed {
		spentBytes, err := spent.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize spent %d for ls-milestone-diff %d: %w", x, md.MilestoneIndex, err)
		}
		if _, err := b.Write(spentBytes); err != nil {
			return nil, fmt.Errorf("unable to write spent %d for ls-milestone-diff %d: %w", x, md.MilestoneIndex, err)
		}
	}

	return b.Bytes(), nil
}

// SEPProducerFunc yields a solid entry point to be written to a snapshot or nil if no more is available.
type SEPProducerFunc func() (*hornet.MessageID, error)

// SEPConsumerFunc consumes the given solid entry point.
// A returned error signals to cancel further reading.
type SEPConsumerFunc func(*hornet.MessageID) error

// HeaderConsumerFunc consumes the snapshot file header.
// A returned error signals to cancel further reading.
type HeaderConsumerFunc func(*ReadFileHeader) error

// OutputProducerFunc yields an output to be written to a snapshot or nil if no more is available.
type OutputProducerFunc func() (*Output, error)

// OutputConsumerFunc consumes the given output.
// A returned error signals to cancel further reading.
type OutputConsumerFunc func(output *Output) error

// MilestoneDiffProducerFunc yields a milestone diff to be written to a snapshot or nil if no more is available.
type MilestoneDiffProducerFunc func() (*MilestoneDiff, error)

// MilestoneDiffConsumerFunc consumes the given MilestoneDiff.
// A returned error signals to cancel further reading.
type MilestoneDiffConsumerFunc func(milestoneDiff *MilestoneDiff) error

// FileHeader is the file header of a snapshot file.
type FileHeader struct {
	// Version denotes the version of this snapshot.
	Version byte
	// Type denotes the type of this snapshot.
	Type Type
	// The ID of the network for which this snapshot is compatible with.
	NetworkID uint64
	// The milestone index of the SEPs for which this snapshot was taken.
	SEPMilestoneIndex milestone.Index
	// The milestone index of the ledger data within the snapshot.
	LedgerMilestoneIndex milestone.Index
}

// ReadFileHeader is a FileHeader but with additional content read from the snapshot.
type ReadFileHeader struct {
	FileHeader
	// The time at which the snapshot was taken.
	Timestamp uint64
	// The count of solid entry points.
	SEPCount uint64
	// The count of outputs. This count is zero if a delta snapshot has been read.
	OutputCount uint64
	// The count of milestone diffs.
	MilestoneDiffCount uint64
}

// StreamSnapshotDataTo streams a snapshot data into the given io.WriteSeeker.
// FileHeader.Type is used to determine whether to write a full or delta snapshot.
// If the type of the snapshot is Full, then OutputProducerFunc must be provided.
func StreamSnapshotDataTo(writeSeeker io.WriteSeeker, timestamp uint64, header *FileHeader,
	sepProd SEPProducerFunc, outputProd OutputProducerFunc, msDiffProd MilestoneDiffProducerFunc) error {

	if header.Type == Full && outputProd == nil {
		return ErrOutputProducerNotProvided
	}

	var sepsCount, outputCount, msDiffCount uint64

	// write LS file version and type
	if _, err := writeSeeker.Write([]byte{header.Version, byte(header.Type)}); err != nil {
		return fmt.Errorf("unable to write LS version and type: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, timestamp); err != nil {
		return fmt.Errorf("unable to write LS timestamp: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.NetworkID); err != nil {
		return fmt.Errorf("unable to write LS network ID: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.SEPMilestoneIndex); err != nil {
		return fmt.Errorf("unable to write LS SEPs milestone index: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.LedgerMilestoneIndex); err != nil {
		return fmt.Errorf("unable to write LS ledger milestone index: %w", err)
	}

	// write count placeholders
	placeholderSpace := iotago.UInt64ByteSize * 3
	if header.Type == Delta {
		placeholderSpace -= iotago.UInt64ByteSize
	}
	if _, err := writeSeeker.Write(make([]byte, placeholderSpace)); err != nil {
		return fmt.Errorf("unable to write LS counter placeholders: %w", err)
	}

	for {
		sep, err := sepProd()
		if err != nil {
			return fmt.Errorf("unable to get next LS SEP #%d: %w", sepsCount+1, err)
		}

		if sep == nil {
			break
		}

		sepsCount++
		if _, err := writeSeeker.Write(sep[:]); err != nil {
			return fmt.Errorf("unable to write LS SEP #%d: %w", sepsCount, err)
		}
	}

	if header.Type == Full {
		for {
			output, err := outputProd()
			if err != nil {
				return fmt.Errorf("unable to get next LS output #%d: %w", outputCount+1, err)
			}

			if output == nil {
				break
			}

			outputCount++
			outputBytes, err := output.MarshalBinary()
			if err != nil {
				return fmt.Errorf("unable to serialize LS output #%d: %w", outputCount, err)
			}
			if _, err := writeSeeker.Write(outputBytes); err != nil {
				return fmt.Errorf("unable to write LS output #%d: %w", outputCount, err)
			}
		}
	}

	for {
		msDiff, err := msDiffProd()
		if err != nil {
			return fmt.Errorf("unable to get next LS milestone diff #%d: %w", msDiffCount+1, err)
		}

		if msDiff == nil {
			break
		}

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

	if header.Type == Full {
		if err := binary.Write(writeSeeker, binary.LittleEndian, outputCount); err != nil {
			return fmt.Errorf("unable to write to LS outputs count: %w", err)
		}
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return fmt.Errorf("unable to write to LS ms-diffs count: %w", err)
	}

	return nil
}

// StreamSnapshotDataFrom consumes a snapshot from the given reader.
// OutputConsumerFunc must not be nil if the snapshot is not a delta snapshot.
func StreamSnapshotDataFrom(reader io.Reader, headerConsumer HeaderConsumerFunc,
	sepConsumer SEPConsumerFunc, outputConsumer OutputConsumerFunc, msDiffConsumer MilestoneDiffConsumerFunc) error {
	readHeader := &ReadFileHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Version); err != nil {
		return fmt.Errorf("unable to read LS version: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Type); err != nil {
		return fmt.Errorf("unable to read LS type: %w", err)
	}

	if readHeader.Type == Full && outputConsumer == nil {
		return ErrOutputConsumerNotProvided
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Timestamp); err != nil {
		return fmt.Errorf("unable to read LS timestamp: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.NetworkID); err != nil {
		return fmt.Errorf("unable to read LS network ID: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.SEPMilestoneIndex); err != nil {
		return fmt.Errorf("unable to read LS SEPs milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.LedgerMilestoneIndex); err != nil {
		return fmt.Errorf("unable to read LS ledger milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.SEPCount); err != nil {
		return fmt.Errorf("unable to read LS SEPs count: %w", err)
	}

	if readHeader.Type == Full {
		if err := binary.Read(reader, binary.LittleEndian, &readHeader.OutputCount); err != nil {
			return fmt.Errorf("unable to read LS outputs count: %w", err)
		}
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.MilestoneDiffCount); err != nil {
		return fmt.Errorf("unable to read LS ms-diff count: %w", err)
	}

	if err := headerConsumer(readHeader); err != nil {
		return err
	}

	for i := uint64(0); i < readHeader.SEPCount; i++ {
		var solidEntryPointMessageID hornet.MessageID
		if _, err := io.ReadFull(reader, solidEntryPointMessageID[:]); err != nil {
			return fmt.Errorf("unable to read LS SEP at pos %d: %w", i, err)
		}
		if err := sepConsumer(&solidEntryPointMessageID); err != nil {
			return fmt.Errorf("SEP consumer error at pos %d: %w", i, err)
		}
	}

	if readHeader.Type == Full {
		for i := uint64(0); i < readHeader.OutputCount; i++ {
			output, err := readOutput(reader)
			if err != nil {
				return fmt.Errorf("at pos %d: %w", i, err)
			}

			if err := outputConsumer(output); err != nil {
				return fmt.Errorf("output consumer error at pos %d: %w", i, err)
			}
		}
	}

	for i := uint64(0); i < readHeader.MilestoneDiffCount; i++ {
		msDiff, err := readMilestoneDiff(reader)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}
		if err := msDiffConsumer(msDiff); err != nil {
			return fmt.Errorf("ms-diff consumer error at pos %d: %w", i, err)
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

	msDiff.Consumed = make([]*Spent, consumedCount)
	for i := uint64(0); i < consumedCount; i++ {
		diffConsumedSpent, err := readSpent(reader)
		if err != nil {
			return nil, fmt.Errorf("(ms-diff consumed-output) at pos %d: %w", i, err)
		}
		msDiff.Consumed[i] = diffConsumedSpent
	}

	return msDiff, nil
}

// reads an Output from the given reader.
func readOutput(reader io.Reader) (*Output, error) {
	output := &Output{}
	if _, err := io.ReadFull(reader, output.MessageID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS message ID: %w", err)
	}

	if _, err := io.ReadFull(reader, output.OutputID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS output ID: %w", err)
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

	if err := binary.Read(reader, binary.LittleEndian, &output.Amount); err != nil {
		return nil, fmt.Errorf("unable to read LS output value: %w", err)
	}

	return output, nil
}

func readSpent(reader io.Reader) (*Spent, error) {
	output, err := readOutput(reader)
	if err != nil {
		return nil, err
	}

	spent := &Spent{Output: *output}
	if _, err := io.ReadFull(reader, spent.TargetTransactionID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS target transaction ID: %w", err)
	}

	return spent, nil
}
