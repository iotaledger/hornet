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
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// The supported snapshot file version.
	SupportedFormatVersion byte = 2
	// The length of a solid entry point hash.
	SolidEntryPointHashLength = iotago.MessageIDLength

	// The offset of counters within a snapshot file:
	// version + type + timestamp + network-id + sep-ms-index + ledger-ms-index
	countersOffset = serializer.OneByte + serializer.OneByte + serializer.UInt64ByteSize + serializer.UInt64ByteSize +
		serializer.UInt32ByteSize + serializer.UInt32ByteSize
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

// LexicalOrderedOutputs are Outputs ordered in lexical order by their OutputID.
type LexicalOrderedOutputs utxo.Outputs

func (l LexicalOrderedOutputs) Len() int {
	return len(l)
}

func (l LexicalOrderedOutputs) Less(i, j int) bool {
	return bytes.Compare(l[i].OutputID()[:], l[j].OutputID()[:]) < 0
}

func (l LexicalOrderedOutputs) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// MilestoneDiff represents the outputs which were created and consumed for the given milestone
// and the message itself which contains the milestone.
type MilestoneDiff struct {
	// The milestone payload itself.
	Milestone *iotago.Milestone
	// The created outputs with this milestone.
	Created utxo.Outputs
	// The consumed spents with this milestone.
	Consumed utxo.Spents
	// The consumed treasury output with this milestone.
	SpentTreasuryOutput *utxo.TreasuryOutput
}

// TreasuryOutput extracts the new treasury output from within the milestone receipt.
// Might return nil if there is no receipt within the milestone.
func (md *MilestoneDiff) TreasuryOutput() *utxo.TreasuryOutput {
	if md.Milestone.Receipt == nil {
		return nil
	}
	to := md.Milestone.Receipt.(*iotago.Receipt).Transaction.Output
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

	msBytes, err := md.Milestone.Serialize(serializer.DeSeriModePerformValidation, nil)
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
		outputBytes := output.SnapshotBytes()
		if _, err := b.Write(outputBytes); err != nil {
			return nil, fmt.Errorf("unable to write output %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Consumed))); err != nil {
		return nil, fmt.Errorf("unable to write consumed outputs array length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	for x, spent := range md.Consumed {
		spentBytes := spent.SnapshotBytes()
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
type OutputProducerFunc func() (*utxo.Output, error)

// OutputConsumerFunc consumes the given output.
// A returned error signals to cancel further reading.
type OutputConsumerFunc func(output *utxo.Output) error

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
	placeholderSpace := serializer.UInt64ByteSize * 3
	if header.Type == Delta {
		placeholderSpace -= serializer.UInt64ByteSize
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
			outputBytes := output.SnapshotBytes()
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
	deSeriParas *iotago.DeSerializationParameters,
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
			output, err := readOutput(reader, deSeriParas)
			if err != nil {
				return fmt.Errorf("at pos %d: %w", i, err)
			}

			if err := outputConsumer(output); err != nil {
				return fmt.Errorf("output consumer error at pos %d: %w", i, err)
			}
		}
	}

	for i := uint64(0); i < readHeader.MilestoneDiffCount; i++ {
		msDiff, err := readMilestoneDiff(reader, deSeriParas)
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
func readMilestoneDiff(reader io.Reader, deSeriParas *iotago.DeSerializationParameters) (*MilestoneDiff, error) {
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

	if _, err := ms.Deserialize(msBytes, serializer.DeSeriModePerformValidation, deSeriParas); err != nil {
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

	msDiff.Created = make(utxo.Outputs, createdCount)
	for i := uint64(0); i < createdCount; i++ {
		diffCreatedOutput, err := readOutput(reader, deSeriParas)
		if err != nil {
			return nil, fmt.Errorf("(ms-diff created-output) at pos %d: %w", i, err)
		}
		msDiff.Created[i] = diffCreatedOutput
	}

	if err := binary.Read(reader, binary.LittleEndian, &consumedCount); err != nil {
		return nil, fmt.Errorf("unable to read LS ms-diff consumed count: %w", err)
	}

	msDiff.Consumed = make(utxo.Spents, consumedCount)
	for i := uint64(0); i < consumedCount; i++ {
		diffConsumedSpent, err := readSpent(reader, deSeriParas, milestone.Index(ms.Index), ms.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("(ms-diff consumed-output) at pos %d: %w", i, err)
		}
		msDiff.Consumed[i] = diffConsumedSpent
	}

	return msDiff, nil
}

// reads an Output from the given reader.
func readOutput(reader io.Reader, deSeriParas *iotago.DeSerializationParameters) (*utxo.Output, error) {
	return utxo.OutputFromSnapshotReader(reader, deSeriParas)
}

func readSpent(reader io.Reader, deSeriParas *iotago.DeSerializationParameters, msIndex milestone.Index, msTimestamp uint64) (*utxo.Spent, error) {
	return utxo.SpentFromSnapshotReader(reader, deSeriParas, msIndex, msTimestamp)
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
