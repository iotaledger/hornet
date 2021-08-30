package snapshot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	iotago "github.com/iotaledger/iota.go/v2"
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
	// Returned when the treasury output for a full snapshot has not been provided.
	ErrTreasuryOutputNotProvided = errors.New("treasury output is not provided")
	// Returned when a treasury output consumer has not been provided.
	ErrTreasuryOutputConsumerNotProvided = errors.New("treasury output consumer is not provided")
	// Returned if specified snapshots are not mergeable.
	ErrSnapshotsNotMergeable = errors.New("snapshot files not mergeable")
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

// maps the snapshot type to its name.
var snapshotNames = map[Type]string{
	Full:  "full",
	Delta: "delta",
}

// Output defines an output within a snapshot.
type Output struct {
	// The message ID of the message that contained the transaction where this output was created.
	MessageID [iotago.MessageIDLength]byte `json:"message_id"`
	// The transaction ID and the index of the output.
	OutputID [iotago.TransactionIDLength + 2]byte `json:"output_id"`
	// The type of the output.
	OutputType iotago.OutputType `json:"output_type"`
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
	if err := b.WriteByte(s.OutputType); err != nil {
		return nil, fmt.Errorf("unable to write output type for ls-output: %w", err)
	}
	addrData, err := s.Address.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize address for ls-output: %w", err)
	}
	if _, err = b.Write(addrData); err != nil {
		return nil, fmt.Errorf("unable to write address for ls-output: %w", err)
	}
	if err = binary.Write(&b, binary.LittleEndian, s.Amount); err != nil {
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

// MilestoneDiff represents the outputs which were created and consumed for the given milestone
// and the message itself which contains the milestone.
type MilestoneDiff struct {
	// The milestone payload itself.
	Milestone *iotago.Milestone `json:"milestone"`
	// The created outputs with this milestone.
	Created []*Output `json:"created"`
	// The consumed spents with this milestone.
	Consumed []*Spent `json:"consumed"`
	// The consumed treasury output with this milestone.
	SpentTreasuryOutput *utxo.TreasuryOutput
}

// TreasuryOutput extracts the new treasury output from within the milestone receipt.
// Might return nil if there is no receipt within the milestone.
func (md *MilestoneDiff) TreasuryOutput() *utxo.TreasuryOutput {
	if md.Milestone.Receipt == nil {
		return nil
	}
	to := md.Milestone.Receipt.(*iotago.Receipt).
		Transaction.(*iotago.TreasuryTransaction).
		Output.(*iotago.TreasuryOutput)
	msID, err := md.Milestone.ID()
	if err != nil {
		panic(err)
	}
	utxoTo := &utxo.TreasuryOutput{Amount: to.Amount}
	copy(utxoTo.MilestoneID[:], msID[:])
	return utxoTo
}

func (md *MilestoneDiff) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer

	msBytes, err := md.Milestone.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize milestone for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	if err := binary.Write(&b, binary.LittleEndian, uint32(len(msBytes))); err != nil {
		return nil, fmt.Errorf("unable to write milestone payload length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	if _, err := b.Write(msBytes); err != nil {
		return nil, fmt.Errorf("unable to write milestone payload for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	// write in spent treasury output
	if md.Milestone.Receipt != nil {
		if md.SpentTreasuryOutput == nil {
			panic("milestone diff includes a receipt but no spent treasury output is set")
		}
		if _, err := b.Write(md.SpentTreasuryOutput.MilestoneID[:]); err != nil {
			return nil, fmt.Errorf("unable to write treasury input milestone hash for ls-milestone-diff %d: %w", md.Milestone.Index, err)
		}

		if err := binary.Write(&b, binary.LittleEndian, md.SpentTreasuryOutput.Amount); err != nil {
			return nil, fmt.Errorf("unable to write treasury input amount for ls-milestone-diff %d: %w", md.Milestone.Index, err)
		}
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Created))); err != nil {
		return nil, fmt.Errorf("unable to write created outputs array length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	for x, output := range md.Created {
		outputBytes, err := output.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize output %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
		if _, err := b.Write(outputBytes); err != nil {
			return nil, fmt.Errorf("unable to write output %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Consumed))); err != nil {
		return nil, fmt.Errorf("unable to write consumed outputs array length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	for x, spent := range md.Consumed {
		spentBytes, err := spent.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize spent %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
		if _, err := b.Write(spentBytes); err != nil {
			return nil, fmt.Errorf("unable to write spent %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
	}

	return b.Bytes(), nil
}

// SEPProducerFunc yields a solid entry point to be written to a snapshot or nil if no more is available.
type SEPProducerFunc func() (hornet.MessageID, error)

// SEPConsumerFunc consumes the given solid entry point.
// A returned error signals to cancel further reading.
type SEPConsumerFunc func(hornet.MessageID) error

// HeaderConsumerFunc consumes the snapshot file header.
// A returned error signals to cancel further reading.
type HeaderConsumerFunc func(*ReadFileHeader) error

// OutputProducerFunc yields an output to be written to a snapshot or nil if no more is available.
type OutputProducerFunc func() (*Output, error)

// OutputConsumerFunc consumes the given output.
// A returned error signals to cancel further reading.
type OutputConsumerFunc func(output *Output) error

// UnspentTreasuryOutputConsumerFunc consumes the given treasury output.
// A returned error signals to cancel further reading.
type UnspentTreasuryOutputConsumerFunc func(output *utxo.TreasuryOutput) error

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
	// The treasury output existing for the given ledger milestone index.
	// This field must be populated if a Full snapshot is created/read.
	TreasuryOutput *utxo.TreasuryOutput
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

// getSnapshotFilesLedgerIndex returns the final ledger index if the given snapshot files would be applied.
func getSnapshotFilesLedgerIndex(fullHeader *ReadFileHeader, deltaHeader *ReadFileHeader) milestone.Index {

	if fullHeader == nil {
		return 0
	}

	if deltaHeader == nil {
		return fullHeader.SEPMilestoneIndex
	}

	return deltaHeader.SEPMilestoneIndex
}

// StreamSnapshotDataTo streams a snapshot data into the given io.WriteSeeker.
// FileHeader.Type is used to determine whether to write a full or delta snapshot.
// If the type of the snapshot is Full, then OutputProducerFunc must be provided.
func StreamSnapshotDataTo(writeSeeker io.WriteSeeker, timestamp uint64, header *FileHeader,
	sepProd SEPProducerFunc, outputProd OutputProducerFunc, msDiffProd MilestoneDiffProducerFunc) (*SnapshotMetrics, error) {

	if header.Type == Full {
		switch {
		case outputProd == nil:
			return nil, ErrOutputProducerNotProvided
		case header.TreasuryOutput == nil:
			return nil, ErrTreasuryOutputNotProvided
		}
	}

	var sepsCount, outputCount, msDiffCount uint64

	timeStart := time.Now()

	// write LS file version and type
	if _, err := writeSeeker.Write([]byte{header.Version, byte(header.Type)}); err != nil {
		return nil, fmt.Errorf("unable to write LS version and type: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, timestamp); err != nil {
		return nil, fmt.Errorf("unable to write LS timestamp: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.NetworkID); err != nil {
		return nil, fmt.Errorf("unable to write LS network ID: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.SEPMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to write LS SEPs milestone index: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, header.LedgerMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to write LS ledger milestone index: %w", err)
	}

	// write count placeholders
	placeholderSpace := iotago.UInt64ByteSize * 3
	if header.Type == Delta {
		placeholderSpace -= iotago.UInt64ByteSize
	}
	if _, err := writeSeeker.Write(make([]byte, placeholderSpace)); err != nil {
		return nil, fmt.Errorf("unable to write LS counter placeholders: %w", err)
	}

	if header.Type == Full {
		if _, err := writeSeeker.Write(header.TreasuryOutput.MilestoneID[:]); err != nil {
			return nil, fmt.Errorf("unable to write LS treasury output milestone hash: %w", err)
		}
		if err := binary.Write(writeSeeker, binary.LittleEndian, header.TreasuryOutput.Amount); err != nil {
			return nil, fmt.Errorf("unable to write LS treasury output amount: %w", err)
		}
	}

	timeHeader := time.Now()

	for {
		sep, err := sepProd()
		if err != nil {
			return nil, fmt.Errorf("unable to get next LS SEP #%d: %w", sepsCount+1, err)
		}

		if sep == nil {
			break
		}

		sepsCount++
		if _, err := writeSeeker.Write(sep[:]); err != nil {
			return nil, fmt.Errorf("unable to write LS SEP #%d: %w", sepsCount, err)
		}
	}

	timeSolidEntryPoints := time.Now()

	if header.Type == Full {
		for {
			output, err := outputProd()
			if err != nil {
				return nil, fmt.Errorf("unable to get next LS output #%d: %w", outputCount+1, err)
			}

			if output == nil {
				break
			}

			outputCount++
			outputBytes, err := output.MarshalBinary()
			if err != nil {
				return nil, fmt.Errorf("unable to serialize LS output #%d: %w", outputCount, err)
			}
			if _, err := writeSeeker.Write(outputBytes); err != nil {
				return nil, fmt.Errorf("unable to write LS output #%d: %w", outputCount, err)
			}
		}
	}

	timeOutputs := time.Now()

	for {
		msDiff, err := msDiffProd()
		if err != nil {
			return nil, fmt.Errorf("unable to get next LS milestone diff #%d: %w", msDiffCount+1, err)
		}

		if msDiff == nil {
			break
		}

		msDiffCount++
		msDiffBytes, err := msDiff.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize LS milestone diff #%d: %w", msDiffCount, err)
		}
		if _, err := writeSeeker.Write(msDiffBytes); err != nil {
			return nil, fmt.Errorf("unable to write LS milestone diff #%d: %w", msDiffCount, err)
		}
	}

	timeMilestoneDiffs := time.Now()

	if _, err := writeSeeker.Seek(countersOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return nil, fmt.Errorf("unable to write to LS SEPs count: %w", err)
	}

	if header.Type == Full {
		if err := binary.Write(writeSeeker, binary.LittleEndian, outputCount); err != nil {
			return nil, fmt.Errorf("unable to write to LS outputs count: %w", err)
		}
	}

	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return nil, fmt.Errorf("unable to write to LS ms-diffs count: %w", err)
	}

	return &SnapshotMetrics{
		DurationHeader:           timeHeader.Sub(timeStart),
		DurationSolidEntryPoints: timeSolidEntryPoints.Sub(timeHeader),
		DurationOutputs:          timeOutputs.Sub(timeSolidEntryPoints),
		DurationMilestoneDiffs:   timeMilestoneDiffs.Sub(timeOutputs),
	}, nil
}

// ReadSnapshotHeader reads the snapshot header from the given reader.
func ReadSnapshotHeader(reader io.Reader) (*ReadFileHeader, error) {
	readHeader := &ReadFileHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Version); err != nil {
		return nil, fmt.Errorf("unable to read LS version: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Type); err != nil {
		return nil, fmt.Errorf("unable to read LS type: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Timestamp); err != nil {
		return nil, fmt.Errorf("unable to read LS timestamp: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.NetworkID); err != nil {
		return nil, fmt.Errorf("unable to read LS network ID: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.SEPMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS SEPs milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.LedgerMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS ledger milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.SEPCount); err != nil {
		return nil, fmt.Errorf("unable to read LS SEPs count: %w", err)
	}

	if readHeader.Type == Full {
		if err := binary.Read(reader, binary.LittleEndian, &readHeader.OutputCount); err != nil {
			return nil, fmt.Errorf("unable to read LS outputs count: %w", err)
		}
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.MilestoneDiffCount); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff count: %w", err)
	}

	if readHeader.Type == Full {
		to := &utxo.TreasuryOutput{Spent: false}
		if _, err := io.ReadFull(reader, to.MilestoneID[:]); err != nil {
			return nil, fmt.Errorf("unable to read LS treasury output milestone hash: %w", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &to.Amount); err != nil {
			return nil, fmt.Errorf("unable to read LS treasury output amount: %w", err)
		}

		readHeader.TreasuryOutput = to
	}

	return readHeader, nil
}

// StreamSnapshotDataFrom consumes a snapshot from the given reader.
// OutputConsumerFunc must not be nil if the snapshot is not a delta snapshot.
func StreamSnapshotDataFrom(reader io.Reader,
	headerConsumer HeaderConsumerFunc,
	sepConsumer SEPConsumerFunc,
	outputConsumer OutputConsumerFunc,
	unspentTreasuryOutputConsumer UnspentTreasuryOutputConsumerFunc,
	msDiffConsumer MilestoneDiffConsumerFunc) error {

	readHeader, err := ReadSnapshotHeader(reader)
	if err != nil {
		return err
	}

	if readHeader.Type == Full {
		switch {
		case outputConsumer == nil:
			return ErrOutputConsumerNotProvided
		case unspentTreasuryOutputConsumer == nil:
			return ErrTreasuryOutputConsumerNotProvided
		}

		if err := unspentTreasuryOutputConsumer(readHeader.TreasuryOutput); err != nil {
			return err
		}
	}

	if err := headerConsumer(readHeader); err != nil {
		return err
	}

	for i := uint64(0); i < readHeader.SEPCount; i++ {
		solidEntryPointMessageID := make(hornet.MessageID, iotago.MessageIDLength)
		if _, err := io.ReadFull(reader, solidEntryPointMessageID); err != nil {
			return fmt.Errorf("unable to read LS SEP at pos %d: %w", i, err)
		}
		if err := sepConsumer(solidEntryPointMessageID); err != nil {
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

	var msLength uint32
	if err := binary.Read(reader, binary.LittleEndian, &msLength); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff ms length: %w", err)
	}

	msBytes := make([]byte, msLength)
	ms := &iotago.Milestone{}
	if _, err := io.ReadFull(reader, msBytes); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff ms: %w", err)
	}

	if _, err := ms.Deserialize(msBytes, iotago.DeSeriModePerformValidation); err != nil {
		return nil, fmt.Errorf("unable to deserialize LS ms-diff ms: %w", err)
	}

	msDiff.Milestone = ms

	if ms.Receipt != nil {
		spentTreasuryOutput := &utxo.TreasuryOutput{Spent: true}
		if _, err := io.ReadFull(reader, spentTreasuryOutput.MilestoneID[:]); err != nil {
			return nil, fmt.Errorf("unable to read LS ms-diff treasury input milestone hash: %w", err)
		}

		if err := binary.Read(reader, binary.LittleEndian, &spentTreasuryOutput.Amount); err != nil {
			return nil, fmt.Errorf("unable to read LS ms-diff treasury input milestone amount: %w", err)
		}

		msDiff.SpentTreasuryOutput = spentTreasuryOutput
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

	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(reader, typeBuf); err != nil {
		return nil, fmt.Errorf("unable to read LS output type: %w", err)
	}
	output.OutputType = typeBuf[0]

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

// ReadSnapshotHeaderFromFile reads the header of the given snapshot file.
func ReadSnapshotHeaderFromFile(filePath string) (*ReadFileHeader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open snapshot file to read header: %w", err)
	}
	defer func() { _ = file.Close() }()

	return ReadSnapshotHeader(file)
}

// MergeInfo holds information about a merge of a full and delta snapshot.
type MergeInfo struct {
	// The header of the full snapshot.
	FullSnapshotHeader *ReadFileHeader
	// The header of the delta snapshot.
	DeltaSnapshotHeader *ReadFileHeader
	// The header of the merged snapshot.
	MergedSnapshotHeader *FileHeader
	// The total output count of the ledger.
	UnspentOutputsCount uint64
	// The total count of solid entry points.
	SEPsCount int
}

// MergeSnapshotsFiles merges the given full and delta snapshots to create an updated full snapshot.
// The result is a full snapshot file containing the ledger outputs corresponding to the
// snapshot index of the specified delta snapshot. The target file does not include any milestone diffs
// and the ledger and snapshot index are equal.
// This function consumes disk space over memory by importing the full snapshot into a temporary database,
// applying the delta diffs onto it and then writing out the merged state.
func MergeSnapshotsFiles(tempDBPath string, fullPath string, deltaPath string, targetFileName string) (*MergeInfo, error) {

	// check that the delta snapshot file's ledger index equals the snapshot index of the full one
	fullHeader, err := ReadSnapshotHeaderFromFile(fullPath)
	if err != nil {
		return nil, err
	}

	deltaHeader, err := ReadSnapshotHeaderFromFile(deltaPath)
	if err != nil {
		return nil, err
	}

	if deltaHeader.LedgerMilestoneIndex != fullHeader.SEPMilestoneIndex {
		return nil, fmt.Errorf("%w: delta snapshot's ledger index %d does not correspond to full snapshot's SEPs index %d",
			ErrSnapshotsNotMergeable, deltaHeader.LedgerMilestoneIndex, fullHeader.SEPMilestoneIndex)
	}

	// spawn temporary database to built up the wanted merged state in
	pebbleDB, err := pebble.CreateDB(tempDBPath)
	if err != nil {
		return nil, err
	}
	kvStore := pebble.New(pebbleDB)

	defer func() {
		// clean up temp db
		kvStore.Shutdown()
		_ = kvStore.Close()
		_ = os.RemoveAll(tempDBPath)
	}()

	fullSnapshotFile, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open full snapshot file: %w", err)
	}
	defer func() { _ = fullSnapshotFile.Close() }()

	// build up retracted ledger state
	mergeUTXOManager := utxo.New(kvStore)
	if err := StreamSnapshotDataFrom(fullSnapshotFile,
		func(header *ReadFileHeader) error {
			return mergeUTXOManager.StoreLedgerIndex(header.LedgerMilestoneIndex)
		},
		func(id hornet.MessageID) error {
			return nil
		}, newOutputConsumer(mergeUTXOManager), newUnspentTreasuryOutputConsumer(mergeUTXOManager), newMsDiffConsumer(mergeUTXOManager),
	); err != nil {
		return nil, fmt.Errorf("unable to import full snapshot data into temp database: %w", err)
	}

	deltaSnapshotFile, err := os.Open(deltaPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open delta snapshot file: %w", err)
	}
	defer func() { _ = deltaSnapshotFile.Close() }()

	// build up ledger state to delta snapshot index
	deltaSnapSEPs := make(hornet.MessageIDs, 0)
	if err := StreamSnapshotDataFrom(deltaSnapshotFile,
		func(header *ReadFileHeader) error {
			return mergeUTXOManager.StoreLedgerIndex(header.LedgerMilestoneIndex)
		}, func(msgID hornet.MessageID) error {
			deltaSnapSEPs = append(deltaSnapSEPs, msgID)
			return nil
		}, nil, nil, newMsDiffConsumer(mergeUTXOManager),
	); err != nil {
		return nil, fmt.Errorf("unable to import delta snapshot data into temp database: %w", err)
	}

	// write out merged state to full snapshot file
	targetSnapshotFile, err := os.Create(targetFileName)
	if err != nil {
		return nil, fmt.Errorf("unable to open target snapshot file: %w", err)
	}
	defer func() { _ = targetSnapshotFile.Close() }()

	var sepsIndex int
	sepsIter := func() (hornet.MessageID, error) {
		if sepsIndex == len(deltaSnapSEPs) {
			return nil, nil
		}
		sep := deltaSnapSEPs[sepsIndex]
		sepsIndex++
		return sep, nil
	}

	// create a prepped output producer which counts how many went through
	var unspentOutputsCount uint64
	cmiUTXOProducer := newCMIUTXOProducer(mergeUTXOManager)
	countingOutputProducer := func() (*Output, error) {
		output, err := cmiUTXOProducer()
		if output != nil {
			unspentOutputsCount++
		}
		return output, err
	}

	unspentTreasuryOutput, err := mergeUTXOManager.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, fmt.Errorf("unable to get final unspent treasury output: %w", err)
	}

	mergedSnapshotFileHeader := &FileHeader{
		Version:           SupportedFormatVersion,
		Type:              Full,
		NetworkID:         deltaHeader.NetworkID,
		SEPMilestoneIndex: deltaHeader.SEPMilestoneIndex,
		// the SEP index on the delta snapshot is the built up state of applying
		// all ms diffs to the origin state of the full snapshot
		LedgerMilestoneIndex: deltaHeader.SEPMilestoneIndex,
		TreasuryOutput:       unspentTreasuryOutput,
	}

	if _, err := StreamSnapshotDataTo(
		targetSnapshotFile, uint64(time.Now().Unix()), mergedSnapshotFileHeader,
		sepsIter, countingOutputProducer,
		func() (*MilestoneDiff, error) {
			// we won't have any ms diffs within this merged full snapshot file
			return nil, nil
		}); err != nil {
		return nil, fmt.Errorf("unable to write merged full snapshot data to target file %s: %w", targetFileName, err)
	}

	return &MergeInfo{
		FullSnapshotHeader:   fullHeader,
		DeltaSnapshotHeader:  deltaHeader,
		MergedSnapshotHeader: mergedSnapshotFileHeader,
		UnspentOutputsCount:  unspentOutputsCount,
		SEPsCount:            len(deltaSnapSEPs),
	}, nil
}
