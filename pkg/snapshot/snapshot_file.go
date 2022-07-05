package snapshot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// SupportedFormatVersion defines the supported snapshot file version.
	SupportedFormatVersion byte = 2
)

var (
	// ErrMilestoneDiffProducerNotProvided is returned when a milestone diff producer has not been provided.
	ErrMilestoneDiffProducerNotProvided = errors.New("milestone diff producer is not provided")
	// ErrSolidEntryPointProducerNotProvided is returned when a solid entry point producer has not been provided.
	ErrSolidEntryPointProducerNotProvided = errors.New("solid entry point producer is not provided")
	// ErrOutputProducerNotProvided is returned when an output producer has not been provided.
	ErrOutputProducerNotProvided = errors.New("output producer is not provided")
	// ErrOutputConsumerNotProvided is returned when an output consumer has not been provided.
	ErrOutputConsumerNotProvided = errors.New("output consumer is not provided")
	// ErrTreasuryOutputNotProvided is returned when the treasury output for a full snapshot has not been provided.
	ErrTreasuryOutputNotProvided = errors.New("treasury output is not provided")
	// ErrTreasuryOutputConsumerNotProvided is returned when a treasury output consumer has not been provided.
	ErrTreasuryOutputConsumerNotProvided = errors.New("treasury output consumer is not provided")
	// ErrSnapshotsNotMergeable is returned if specified snapshots are not mergeable.
	ErrSnapshotsNotMergeable = errors.New("snapshot files not mergeable")
	// ErrWrongSnapshotType is returned if the snapshot type is not supported by this function.
	ErrWrongSnapshotType = errors.New("wrong snapshot type")
)

// Type defines the type of the snapshot.
type Type byte

const (
	// Full is a snapshot which contains the full ledger entry for a given milestone
	// plus the milestone diffs which subtracted to the ledger milestone reduce to the target milestone ledger.
	// the full snapshot contains additional milestone diffs to calculate the correct protocol parameters (before the target index).
	Full Type = iota
	// Delta is a snapshot which contains solely diffs of milestones newer than a certain ledger milestone
	// instead of the complete ledger state of a given milestone.
	// the delta snapshot contains no additional milestone diffs to calculate the correct protocol parameters,
	// because they are they are already included in the full snapshot.
	Delta
)

// maps the snapshot type to its name.
var snapshotNames = map[Type]string{
	Full:  "full",
	Delta: "delta",
}

// ReadWriteTruncateSeeker is the interface used to read, write and truncate a file.
type ReadWriteTruncateSeeker interface {
	io.ReadWriteSeeker
	Truncate(size int64) error
}

func increaseOffsets(amount int64, offsets ...*int64) {
	for _, offset := range offsets {
		*offset += amount
	}
}

// MilestoneDiff represents the outputs which were created and consumed for the given milestone
// and the block itself which contains the milestone.
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
	receipt := md.Milestone.Opts.MustSet().Receipt()
	if receipt == nil {
		return nil
	}
	to := receipt.Transaction.Output
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

	var msDiffLength int64
	// we increase the offsets here, but we add the bytes at the end because we can not seek in the buffer
	increaseOffsets(serializer.UInt32ByteSize, &msDiffLength)

	msBytes, err := md.Milestone.Serialize(serializer.DeSeriModePerformValidation, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize milestone for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	if err := binary.Write(&b, binary.LittleEndian, uint32(len(msBytes))); err != nil {
		return nil, fmt.Errorf("unable to write milestone payload length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &msDiffLength)

	if _, err := b.Write(msBytes); err != nil {
		return nil, fmt.Errorf("unable to write milestone payload for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}
	increaseOffsets(int64(len(msBytes)), &msDiffLength)

	// write in spent treasury output
	opts := md.Milestone.Opts.MustSet()
	if opts.Receipt() != nil {
		if md.SpentTreasuryOutput == nil {
			panic("milestone diff includes a receipt but no spent treasury output is set")
		}
		if _, err := b.Write(md.SpentTreasuryOutput.MilestoneID[:]); err != nil {
			return nil, fmt.Errorf("unable to write treasury input milestone hash for ls-milestone-diff %d: %w", md.Milestone.Index, err)
		}
		increaseOffsets(iotago.MilestoneIDLength, &msDiffLength)

		if err := binary.Write(&b, binary.LittleEndian, md.SpentTreasuryOutput.Amount); err != nil {
			return nil, fmt.Errorf("unable to write treasury input amount for ls-milestone-diff %d: %w", md.Milestone.Index, err)
		}
		increaseOffsets(serializer.UInt64ByteSize, &msDiffLength)
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Created))); err != nil {
		return nil, fmt.Errorf("unable to write created outputs array length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}
	increaseOffsets(serializer.UInt64ByteSize, &msDiffLength)

	for x, output := range md.Created {
		outputBytes := output.SnapshotBytes()
		if _, err := b.Write(outputBytes); err != nil {
			return nil, fmt.Errorf("unable to write output %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
		increaseOffsets(int64(len(outputBytes)), &msDiffLength)
	}

	if err := binary.Write(&b, binary.LittleEndian, uint64(len(md.Consumed))); err != nil {
		return nil, fmt.Errorf("unable to write consumed outputs array length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}
	increaseOffsets(serializer.UInt64ByteSize, &msDiffLength)

	for x, spent := range md.Consumed {
		spentBytes := spent.SnapshotBytes()
		if _, err := b.Write(spentBytes); err != nil {
			return nil, fmt.Errorf("unable to write spent %d for ls-milestone-diff %d: %w", x, md.Milestone.Index, err)
		}
		increaseOffsets(int64(len(spentBytes)), &msDiffLength)
	}

	var bufMilestoneDiffLengthOffset bytes.Buffer
	if err := binary.Write(&bufMilestoneDiffLengthOffset, binary.LittleEndian, uint32(msDiffLength)); err != nil {
		return nil, fmt.Errorf("unable to write length for ls-milestone-diff %d: %w", md.Milestone.Index, err)
	}

	return append(bufMilestoneDiffLengthOffset.Bytes(), b.Bytes()...), nil
}

// reads a MilestoneDiff from the given reader.
func ReadMilestoneDiff(reader io.ReadSeeker, protocolStorage *storage.ProtocolStorage, addProtocolParameterUpdates bool) (int64, *MilestoneDiff, error) {
	msDiff := &MilestoneDiff{}

	var msDiffLength uint32
	if err := binary.Read(reader, binary.LittleEndian, &msDiffLength); err != nil {
		return 0, nil, fmt.Errorf("unable to read LS ms-diff length: %w", err)
	}

	var msLength uint32
	if err := binary.Read(reader, binary.LittleEndian, &msLength); err != nil {
		return 0, nil, fmt.Errorf("unable to read LS ms-diff ms length: %w", err)
	}

	msBytes := make([]byte, msLength)
	milestonePayload := &iotago.Milestone{}
	if _, err := io.ReadFull(reader, msBytes); err != nil {
		return 0, nil, fmt.Errorf("unable to read LS ms-diff ms: %w", err)
	}

	if _, err := milestonePayload.Deserialize(msBytes, serializer.DeSeriModePerformValidation, nil); err != nil {
		return 0, nil, fmt.Errorf("unable to deserialize LS ms-diff ms: %w", err)
	}

	msDiff.Milestone = milestonePayload

	if milestonePayload.Opts.MustSet().ProtocolParams() != nil && addProtocolParameterUpdates {
		protocolStorage.StoreProtocolParametersMilestoneOption(milestonePayload.Opts.MustSet().ProtocolParams())
	}

	protoParams, err := protocolStorage.ProtocolParameters(msDiff.Milestone.Index)
	if err != nil {
		return 0, nil, fmt.Errorf("unable to load LS ms-diff protocol parameters: %w", err)
	}

	if milestonePayload.Opts.MustSet().Receipt() != nil {
		spentTreasuryOutput := &utxo.TreasuryOutput{Spent: true}
		if _, err := io.ReadFull(reader, spentTreasuryOutput.MilestoneID[:]); err != nil {
			return 0, nil, fmt.Errorf("unable to read LS ms-diff treasury input milestone hash: %w", err)
		}

		if err := binary.Read(reader, binary.LittleEndian, &spentTreasuryOutput.Amount); err != nil {
			return 0, nil, fmt.Errorf("unable to read LS ms-diff treasury input milestone amount: %w", err)
		}

		msDiff.SpentTreasuryOutput = spentTreasuryOutput
	}

	var createdCount, consumedCount uint64
	if err := binary.Read(reader, binary.LittleEndian, &createdCount); err != nil {
		return 0, nil, fmt.Errorf("unable to read LS ms-diff created count: %w", err)
	}

	msDiff.Created = make(utxo.Outputs, createdCount)
	for i := uint64(0); i < createdCount; i++ {
		diffCreatedOutput, err := ReadOutput(reader, protoParams)
		if err != nil {
			return 0, nil, fmt.Errorf("(ms-diff created-output) at pos %d: %w", i, err)
		}
		msDiff.Created[i] = diffCreatedOutput
	}

	if err := binary.Read(reader, binary.LittleEndian, &consumedCount); err != nil {
		return 0, nil, fmt.Errorf("unable to read LS ms-diff consumed count: %w", err)
	}

	msDiff.Consumed = make(utxo.Spents, consumedCount)
	for i := uint64(0); i < consumedCount; i++ {
		diffConsumedSpent, err := readSpent(reader, protoParams, milestonePayload.Index, milestonePayload.Timestamp)
		if err != nil {
			return 0, nil, fmt.Errorf("(ms-diff consumed-output) at pos %d: %w", i, err)
		}
		msDiff.Consumed[i] = diffConsumedSpent
	}

	return int64(msDiffLength), msDiff, nil
}

// reads protocol parameter updates from a MilestoneDiff from the given reader.
// automatically seek to the end of the MilestoneDiff.
func ReadMilestoneDiffProtocolParameters(reader io.ReadSeeker, protocolStorage *storage.ProtocolStorage) (int64, error) {

	var msDiffLength uint32
	if err := binary.Read(reader, binary.LittleEndian, &msDiffLength); err != nil {
		return 0, fmt.Errorf("unable to read LS ms-diff length: %w", err)
	}

	var msLength uint32
	if err := binary.Read(reader, binary.LittleEndian, &msLength); err != nil {
		return 0, fmt.Errorf("unable to read LS ms-diff ms length: %w", err)
	}

	msBytes := make([]byte, msLength)
	milestonePayload := &iotago.Milestone{}
	if _, err := io.ReadFull(reader, msBytes); err != nil {
		return 0, fmt.Errorf("unable to read LS ms-diff ms: %w", err)
	}

	if _, err := milestonePayload.Deserialize(msBytes, serializer.DeSeriModePerformValidation, nil); err != nil {
		return 0, fmt.Errorf("unable to deserialize LS ms-diff ms: %w", err)
	}

	if milestonePayload.Opts.MustSet().ProtocolParams() != nil {
		protocolStorage.StoreProtocolParametersMilestoneOption(milestonePayload.Opts.MustSet().ProtocolParams())
	}

	// seek to the end of the MilestoneDiff
	// msDiffLength - msDiffLengthSize - msLengthSize - msLength
	if _, err := reader.Seek(int64(msDiffLength-serializer.UInt32ByteSize-serializer.UInt32ByteSize-msLength), io.SeekCurrent); err != nil {
		return 0, err
	}

	return int64(msDiffLength), nil
}

// SEPProducerFunc yields a solid entry point to be written to a snapshot or nil if no more is available.
type SEPProducerFunc func() (iotago.BlockID, error)

// SEPConsumerFunc consumes the given solid entry point.
// A returned error signals to cancel further reading.
type SEPConsumerFunc func(iotago.BlockID, iotago.MilestoneIndex) error

// FullHeaderConsumerFunc consumes the full snapshot file header.
// A returned error signals to cancel further reading.
type FullHeaderConsumerFunc func(h *FullSnapshotHeader) error

// DeltaHeaderConsumerFunc consumes the delta snapshot file header.
// A returned error signals to cancel further reading.
type DeltaHeaderConsumerFunc func(h *DeltaSnapshotHeader) error

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

type FullSnapshotHeader struct {
	// Version denotes the version of this snapshot.
	Version byte
	// Type denotes the type of this snapshot.
	Type Type
	// The index of the genesis milestone of the network.
	GenesisMilestoneIndex iotago.MilestoneIndex
	// The index of the milestone of which the SEPs within the snapshot are from.
	TargetMilestoneIndex iotago.MilestoneIndex
	// The timestamp of the milestone of which the SEPs within the snapshot are from.
	TargetMilestoneTimestamp uint32
	// The ID of the milestone of which the SEPs within the snapshot are from.
	TargetMilestoneID iotago.MilestoneID
	// The index of the milestone of which the UTXOs within the snapshot are from.
	LedgerMilestoneIndex iotago.MilestoneIndex
	// The treasury output existing for the given ledger milestone index.
	// This field must be populated if a Full snapshot is created/read.
	TreasuryOutput *utxo.TreasuryOutput
	// Active Protocol Parameter of the ledger milestone index.
	ProtocolParamsMilestoneOpt *iotago.ProtocolParamsMilestoneOpt
	// The amount of UTXOs contained within this snapshot.
	OutputCount uint64
	// The amount of milestone diffs contained within this snapshot.
	MilestoneDiffCount uint32
	// The amount of SEPs contained within this snapshot.
	SEPCount uint16
}

func (h *FullSnapshotHeader) ProtocolParameters() (*iotago.ProtocolParameters, error) {

	protoParams := &iotago.ProtocolParameters{}
	if _, err := protoParams.Deserialize(h.ProtocolParamsMilestoneOpt.Params, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, fmt.Errorf("failed to deserialize protocol parameters: %w", err)
	}

	return protoParams, nil
}

func writeFullSnapshotHeader(writeSeeker io.WriteSeeker, header *FullSnapshotHeader) (int64, error) {

	if header.Type != Full {
		return 0, ErrWrongSnapshotType
	}
	if header.ProtocolParamsMilestoneOpt == nil {
		return 0, iotago.ErrMissingProtocolParas
	}
	if header.TreasuryOutput == nil {
		return 0, ErrTreasuryOutputNotProvided
	}

	var countersFileOffset int64

	// Version
	// Denotes the version of this file format.
	if _, err := writeSeeker.Write([]byte{header.Version}); err != nil {
		return 0, fmt.Errorf("unable to write LS version: %w", err)
	}
	increaseOffsets(serializer.OneByte, &countersFileOffset)

	// Type
	// Denotes the type of this file format. Value 0 denotes a full snapshot.
	if _, err := writeSeeker.Write([]byte{byte(Full)}); err != nil {
		return 0, fmt.Errorf("unable to write LS type: %w", err)
	}
	increaseOffsets(serializer.OneByte, &countersFileOffset)

	// Genesis Milestone Index
	// The index of the genesis milestone of the network.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.GenesisMilestoneIndex); err != nil {
		return 0, fmt.Errorf("unable to write LS genesis milestone index: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &countersFileOffset)

	// Target Milestone Index
	// The index of the milestone of which the SEPs within the snapshot are from.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.TargetMilestoneIndex); err != nil {
		return 0, fmt.Errorf("unable to write LS target milestone index: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &countersFileOffset)

	// Target Milestone Timestamp
	// The timestamp of the milestone of which the SEPs within the snapshot are from.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.TargetMilestoneTimestamp); err != nil {
		return 0, fmt.Errorf("unable to write LS target milestone timestamp: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &countersFileOffset)

	// Target Milestone ID
	// The ID of the milestone of which the SEPs within the snapshot are from.
	if _, err := writeSeeker.Write(header.TargetMilestoneID[:]); err != nil {
		return 0, fmt.Errorf("unable to write LS target milestone ID: %w", err)
	}
	increaseOffsets(iotago.MilestoneIDLength, &countersFileOffset)

	// Ledger Milestone Index
	// The index of the milestone of which the UTXOs within the snapshot are from.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.LedgerMilestoneIndex); err != nil {
		return 0, fmt.Errorf("unable to write LS ledger milestone index: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &countersFileOffset)

	// Treasury Output Milestone ID
	// The milestone ID of the milestone which generated the treasury output.
	if _, err := writeSeeker.Write(header.TreasuryOutput.MilestoneID[:]); err != nil {
		return 0, fmt.Errorf("unable to write LS treasury output milestone ID: %w", err)
	}
	increaseOffsets(iotago.MilestoneIDLength, &countersFileOffset)

	// Treasury Output Amount
	// The amount of funds residing on the treasury output.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.TreasuryOutput.Amount); err != nil {
		return 0, fmt.Errorf("unable to write LS treasury output amount: %w", err)
	}
	increaseOffsets(serializer.UInt64ByteSize, &countersFileOffset)

	// ProtocolParamsMilestoneOpt Length
	// Denotes the length of the ProtocolParamsMilestoneOpt.
	protoParamsMsOptionBytes, err := header.ProtocolParamsMilestoneOpt.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return 0, fmt.Errorf("unable to serialize LS protocol parameters milestone option: %w", err)
	}
	if err := binary.Write(writeSeeker, binary.LittleEndian, uint16(len(protoParamsMsOptionBytes))); err != nil {
		return 0, fmt.Errorf("unable to write LS protocol parameters milestone option length: %w", err)
	}
	increaseOffsets(serializer.UInt16ByteSize, &countersFileOffset)

	// ProtocolParamsMilestoneOpt
	// Active ProtocolParamsMilestoneOpt of the ledger milestone
	if _, err := writeSeeker.Write(protoParamsMsOptionBytes); err != nil {
		return 0, fmt.Errorf("unable to write LS protocol parameters milestone option: %w", err)
	}
	increaseOffsets(int64(len(protoParamsMsOptionBytes)), &countersFileOffset)

	var outputCount uint64
	var msDiffCount uint32
	var sepsCount uint16

	// Outputs Count
	// The amount of UTXOs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, outputCount); err != nil {
		return 0, fmt.Errorf("unable to write LS outputs count: %w", err)
	}

	// Milestone Diffs Count
	// The amount of milestone diffs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return 0, fmt.Errorf("unable to write LS milestone diffs count: %w", err)
	}

	// SEPs Count
	// The amount of SEPs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return 0, fmt.Errorf("unable to write LS solid entry points count: %w", err)
	}

	return countersFileOffset, nil
}

// ReadFullSnapshotHeader reads the full snapshot header from the given reader.
func ReadFullSnapshotHeader(reader io.Reader) (*FullSnapshotHeader, error) {
	readHeader := &FullSnapshotHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Version); err != nil {
		return nil, fmt.Errorf("unable to read LS version: %w", err)
	}

	if readHeader.Version != SupportedFormatVersion {
		return nil, ErrUnsupportedSnapshot
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.Type); err != nil {
		return nil, fmt.Errorf("unable to read LS type: %w", err)
	}

	if readHeader.Type != Full {
		return nil, ErrUnsupportedSnapshot
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.GenesisMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS genesis milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.TargetMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS target milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.TargetMilestoneTimestamp); err != nil {
		return nil, fmt.Errorf("unable to read LS target milestone timestamp: %w", err)
	}

	if _, err := io.ReadFull(reader, readHeader.TargetMilestoneID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS target milestone ID: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.LedgerMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS ledger milestone index: %w", err)
	}

	to := &utxo.TreasuryOutput{Spent: false}
	if _, err := io.ReadFull(reader, to.MilestoneID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS treasury output milestone ID: %w", err)
	}
	if err := binary.Read(reader, binary.LittleEndian, &to.Amount); err != nil {
		return nil, fmt.Errorf("unable to read LS treasury output amount: %w", err)
	}
	readHeader.TreasuryOutput = to

	var protoParamsMsOptionLength uint16 = 0
	if err := binary.Read(reader, binary.LittleEndian, &protoParamsMsOptionLength); err != nil {
		return nil, fmt.Errorf("unable to read LS protocol parameters milestone option length: %w", err)
	}

	protoParamsMsOptionBytes := make([]byte, protoParamsMsOptionLength)
	if _, err := reader.Read(protoParamsMsOptionBytes); err != nil {
		return nil, fmt.Errorf("unable to read LS protocol parameters milestone option: %w", err)
	}

	readHeader.ProtocolParamsMilestoneOpt = &iotago.ProtocolParamsMilestoneOpt{}
	if _, err := readHeader.ProtocolParamsMilestoneOpt.Deserialize(protoParamsMsOptionBytes, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, fmt.Errorf("unable to deserialize LS protocol parameters milestone option: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.OutputCount); err != nil {
		return nil, fmt.Errorf("unable to read LS outputs count: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.MilestoneDiffCount); err != nil {
		return nil, fmt.Errorf("unable to read LS milestone diffs count: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &readHeader.SEPCount); err != nil {
		return nil, fmt.Errorf("unable to read LS solid entry points count: %w", err)
	}

	return readHeader, nil
}

type DeltaSnapshotHeader struct {
	// Version denotes the version of this snapshot.
	Version byte
	// Type denotes the type of this snapshot.
	Type Type
	// The index of the milestone of which the SEPs within the snapshot are from.
	TargetMilestoneIndex iotago.MilestoneIndex
	// The timestamp of the milestone of which the SEPs within the snapshot are from.
	TargetMilestoneTimestamp uint32
	// The ID of the target milestone of the full snapshot this delta snapshot builts up from.
	FullSnapshotTargetMilestoneID iotago.MilestoneID
	// The file offset of the SEPs field. This is used to easily update an existing delta snapshot without parsing its content.
	SEPFileOffset int64
	// The amount of milestone diffs contained within this snapshot.
	MilestoneDiffCount uint32
	// The amount of SEPs contained within this snapshot.
	SEPCount uint16
}

func writeDeltaSnapshotHeader(writeSeeker io.WriteSeeker, header *DeltaSnapshotHeader) (int64, int64, error) {
	if header.Type != Delta {
		return 0, 0, ErrWrongSnapshotType
	}

	var sepFileOffset int64
	var countersFileOffset int64

	// Version
	// Denotes the version of this file format.
	if _, err := writeSeeker.Write([]byte{header.Version}); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS version: %w", err)
	}
	increaseOffsets(serializer.OneByte, &countersFileOffset, &sepFileOffset)

	// Type
	// Denotes the type of this file format. Value 1 denotes a delta snapshot.
	if _, err := writeSeeker.Write([]byte{byte(Delta)}); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS type: %w", err)
	}
	increaseOffsets(serializer.OneByte, &countersFileOffset, &sepFileOffset)

	// Target Milestone Index
	// The index of the milestone of which the SEPs within the snapshot are from.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.TargetMilestoneIndex); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS target milestone index: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &countersFileOffset, &sepFileOffset)

	// Target Milestone Timestamp
	// The timestamp of the milestone of which the SEPs within the snapshot are from.
	if err := binary.Write(writeSeeker, binary.LittleEndian, header.TargetMilestoneTimestamp); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS target milestone timestamp: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &countersFileOffset, &sepFileOffset)

	// Full Snapshot Target Milestone ID
	// The ID of the target milestone of the full snapshot this delta snapshot builts up from.
	if _, err := writeSeeker.Write(header.FullSnapshotTargetMilestoneID[:]); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS full snapshot target milestone ID: %w", err)
	}
	increaseOffsets(iotago.MilestoneIDLength, &countersFileOffset, &sepFileOffset)

	// SEP File Offset
	// The file offset of the SEPs field. This is used to easily update an existing delta snapshot without parsing its content.
	if err := binary.Write(writeSeeker, binary.LittleEndian, sepFileOffset); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS SEP file offset: %w", err)
	}
	increaseOffsets(serializer.Int64ByteSize, &sepFileOffset)

	var msDiffCount uint32
	var sepsCount uint16

	// Milestone Diffs Count
	// The amount of milestone diffs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS milestone diffs count: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &sepFileOffset)

	// SEPs Count
	// The amount of SEPs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return 0, 0, fmt.Errorf("unable to write LS solid entry points count: %w", err)
	}
	increaseOffsets(serializer.UInt16ByteSize, &sepFileOffset)

	return countersFileOffset, sepFileOffset, nil
}

// ReadDeltaSnapshotHeader reads the delta snapshot header from the given reader.
func ReadDeltaSnapshotHeader(reader io.Reader) (*DeltaSnapshotHeader, error) {
	deltaHeader := &DeltaSnapshotHeader{}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.Version); err != nil {
		return nil, fmt.Errorf("unable to read LS version: %w", err)
	}

	if deltaHeader.Version != SupportedFormatVersion {
		return nil, ErrUnsupportedSnapshot
	}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.Type); err != nil {
		return nil, fmt.Errorf("unable to read LS type: %w", err)
	}

	if deltaHeader.Type != Delta {
		return nil, ErrUnsupportedSnapshot
	}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.TargetMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS target milestone index: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.TargetMilestoneTimestamp); err != nil {
		return nil, fmt.Errorf("unable to read LS target milestone timestamp: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, deltaHeader.FullSnapshotTargetMilestoneID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS full snapshot target milestone ID: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.SEPFileOffset); err != nil {
		return nil, fmt.Errorf("unable to read LS SEP file offset: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.MilestoneDiffCount); err != nil {
		return nil, fmt.Errorf("unable to read LS milestone diffs count: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &deltaHeader.SEPCount); err != nil {
		return nil, fmt.Errorf("unable to read LS solid entry points count: %w", err)
	}

	return deltaHeader, nil
}

// getSnapshotFilesLedgerIndex returns the final ledger index if the given snapshot files would be applied.
func getSnapshotFilesLedgerIndex(fullHeader *FullSnapshotHeader, deltaHeader *DeltaSnapshotHeader) iotago.MilestoneIndex {

	if fullHeader == nil {
		return 0
	}

	if deltaHeader == nil {
		return fullHeader.TargetMilestoneIndex
	}

	return deltaHeader.TargetMilestoneIndex
}

// StreamFullSnapshotDataTo streams a full snapshot data into the given io.WriteSeeker.
// This function modifies the counts in the FullSnapshotHeader.
func StreamFullSnapshotDataTo(
	writeSeeker io.WriteSeeker,
	header *FullSnapshotHeader,
	outputProd OutputProducerFunc,
	msDiffProd MilestoneDiffProducerFunc,
	sepProd SEPProducerFunc) (*SnapshotMetrics, error) {

	if outputProd == nil {
		return nil, ErrOutputProducerNotProvided
	}
	if msDiffProd == nil {
		return nil, ErrMilestoneDiffProducerNotProvided
	}
	if sepProd == nil {
		return nil, ErrSolidEntryPointProducerNotProvided
	}

	timeStart := time.Now()

	countersFileOffset, err := writeFullSnapshotHeader(writeSeeker, header)
	if err != nil {
		return nil, err
	}

	var outputCount uint64
	var msDiffCount uint32
	var sepsCount uint16

	timeHeader := time.Now()

	// Outputs
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
	timeOutputs := time.Now()

	// Milestone Diffs
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

	// SEPs
	for {
		sep, err := sepProd()
		if err != nil {
			if errors.Is(err, ErrNoMoreSEPToProduce) {
				break
			}
			return nil, fmt.Errorf("unable to get next LS SEP #%d: %w", sepsCount+1, err)
		}

		sepsCount++
		if _, err := writeSeeker.Write(sep[:]); err != nil {
			return nil, fmt.Errorf("unable to write LS SEP #%d: %w", sepsCount, err)
		}
	}
	timeSolidEntryPoints := time.Now()

	// seek back to the file position of the counters
	if _, err := writeSeeker.Seek(countersFileOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}

	// Outputs Count
	// The amount of UTXOs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, outputCount); err != nil {
		return nil, fmt.Errorf("unable to write LS outputs count: %w", err)
	}

	// Milestone Diffs Count
	// The amount of milestone diffs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return nil, fmt.Errorf("unable to write LS milestone diffs count: %w", err)
	}

	// SEPs Count
	// The amount of SEPs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return nil, fmt.Errorf("unable to write LS solid entry points count: %w", err)
	}

	// update the values in the header
	header.OutputCount = outputCount
	header.MilestoneDiffCount = msDiffCount
	header.SEPCount = sepsCount

	return &SnapshotMetrics{
		DurationHeader:           timeHeader.Sub(timeStart),
		DurationOutputs:          timeOutputs.Sub(timeHeader),
		DurationMilestoneDiffs:   timeMilestoneDiffs.Sub(timeOutputs),
		DurationSolidEntryPoints: timeSolidEntryPoints.Sub(timeMilestoneDiffs),
	}, nil
}

// StreamDeltaSnapshotDataTo streams delta snapshot data into the given io.WriteSeeker.
func StreamDeltaSnapshotDataTo(
	writeSeeker io.WriteSeeker,
	header *DeltaSnapshotHeader,
	msDiffProd MilestoneDiffProducerFunc,
	sepProd SEPProducerFunc) (*SnapshotMetrics, error) {

	if msDiffProd == nil {
		return nil, ErrMilestoneDiffProducerNotProvided
	}
	if sepProd == nil {
		return nil, ErrSolidEntryPointProducerNotProvided
	}

	timeStart := time.Now()

	countersFileOffset, sepFileOffset, err := writeDeltaSnapshotHeader(writeSeeker, header)
	if err != nil {
		return nil, err
	}

	timeHeader := time.Now()

	var msDiffCount uint32
	var sepsCount uint16

	// Milestone Diffs
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
		increaseOffsets(int64(len(msDiffBytes)), &sepFileOffset)
	}
	timeMilestoneDiffs := time.Now()

	// SEPs
	for {
		sep, err := sepProd()
		if err != nil {
			if errors.Is(err, ErrNoMoreSEPToProduce) {
				break
			}
			return nil, fmt.Errorf("unable to get next LS SEP #%d: %w", sepsCount+1, err)
		}

		sepsCount++
		if _, err := writeSeeker.Write(sep[:]); err != nil {
			return nil, fmt.Errorf("unable to write LS SEP #%d: %w", sepsCount, err)
		}
	}
	timeSolidEntryPoints := time.Now()

	// seek back to the file position of the counters
	if _, err := writeSeeker.Seek(countersFileOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}

	// SEP File Offset
	// The file offset of the SEPs field. This is used to easily update an existing delta snapshot without parsing its content.
	if err := binary.Write(writeSeeker, binary.LittleEndian, sepFileOffset); err != nil {
		return nil, fmt.Errorf("unable to write LS solid entry points file offset: %w", err)
	}

	// Milestone Diffs Count
	// The amount of milestone diffs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, msDiffCount); err != nil {
		return nil, fmt.Errorf("unable to write LS milestone diffs count: %w", err)
	}

	// SEPs Count
	// The amount of SEPs contained within this snapshot.
	if err := binary.Write(writeSeeker, binary.LittleEndian, sepsCount); err != nil {
		return nil, fmt.Errorf("unable to write LS solid entry points count: %w", err)
	}

	// update the values in the header
	header.SEPFileOffset = sepFileOffset
	header.MilestoneDiffCount = msDiffCount
	header.SEPCount = sepsCount

	return &SnapshotMetrics{
		DurationHeader:           timeHeader.Sub(timeStart),
		DurationMilestoneDiffs:   timeMilestoneDiffs.Sub(timeHeader),
		DurationSolidEntryPoints: timeSolidEntryPoints.Sub(timeMilestoneDiffs),
	}, nil
}

// StreamDeltaSnapshotDataToExisting updates a delta snapshot and streams data into the given io.WriteSeeker.
func StreamDeltaSnapshotDataToExisting(
	fileHandle ReadWriteTruncateSeeker,
	header *DeltaSnapshotHeader,
	msDiffProd MilestoneDiffProducerFunc,
	sepProd SEPProducerFunc) (*SnapshotMetrics, error) {

	if header.Type != Delta {
		return nil, ErrWrongSnapshotType
	}
	if msDiffProd == nil {
		return nil, ErrMilestoneDiffProducerNotProvided
	}
	if sepProd == nil {
		return nil, ErrSolidEntryPointProducerNotProvided
	}

	oldDeltaHeader, err := ReadDeltaSnapshotHeader(fileHandle)
	if err != nil {
		return nil, fmt.Errorf("unable to read existing delta snapshot header: %w", err)
	}

	// seek back to the start of the header
	fileHandle.Seek(0, io.SeekStart)

	if oldDeltaHeader.Version != header.Version {
		return nil, errors.New("unable to update existing delta snapshot: mismatching snapshot file version")
	}

	if oldDeltaHeader.FullSnapshotTargetMilestoneID != header.FullSnapshotTargetMilestoneID {
		return nil, fmt.Errorf("unable to update existing delta snapshot: mismatching full snapshot target milestone ID (%s != %s)", oldDeltaHeader.FullSnapshotTargetMilestoneID.ToHex(), header.FullSnapshotTargetMilestoneID.ToHex())
	}

	timeStart := time.Now()
	var fileOffset int64
	var countersFileOffset int64

	// Version
	// Denotes the version of this file format.
	increaseOffsets(serializer.OneByte, &fileOffset, &countersFileOffset)

	// Type
	// Denotes the type of this file format. Value 1 denotes a delta snapshot.
	increaseOffsets(serializer.OneByte, &fileOffset, &countersFileOffset)

	// Seek to the position of Target Milestone Index
	fileHandle.Seek(fileOffset, io.SeekStart)

	// Target Milestone Index
	// The index of the milestone of which the SEPs within the snapshot are from.
	if err := binary.Write(fileHandle, binary.LittleEndian, header.TargetMilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to write LS target milestone index: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &fileOffset, &countersFileOffset)

	// Target Milestone Timestamp
	// The timestamp of the milestone of which the SEPs within the snapshot are from.
	if err := binary.Write(fileHandle, binary.LittleEndian, header.TargetMilestoneTimestamp); err != nil {
		return nil, fmt.Errorf("unable to write LS target milestone timestamp: %w", err)
	}
	increaseOffsets(serializer.UInt32ByteSize, &fileOffset, &countersFileOffset)

	// Full Snapshot Target Milestone ID
	// The ID of the target milestone of the full snapshot this delta snapshot builts up from.
	increaseOffsets(iotago.MilestoneIDLength, &fileOffset, &countersFileOffset)

	timeHeader := time.Now()

	sepFileOffset := oldDeltaHeader.SEPFileOffset
	msDiffCount := oldDeltaHeader.MilestoneDiffCount
	var sepsCount uint16

	// Seek to the position of Target Milestone Index
	fileHandle.Seek(oldDeltaHeader.SEPFileOffset, io.SeekStart)

	// Truncate the old SEPs
	fileHandle.Truncate(oldDeltaHeader.SEPFileOffset)

	// Milestone Diffs
	for {
		msDiff, err := msDiffProd()
		if err != nil {
			return nil, fmt.Errorf("unable to get next LS milestone diff #%d: %w", msDiffCount+1, err)
		}

		if msDiff == nil {
			break
		}

		if msDiff.Milestone.Index <= oldDeltaHeader.TargetMilestoneIndex {
			return nil, fmt.Errorf("milestone diff #%d index is older than the old target index: %d<%d", msDiffCount+1, msDiff.Milestone.Index, oldDeltaHeader.TargetMilestoneIndex)
		}

		msDiffCount++
		msDiffBytes, err := msDiff.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize LS milestone diff #%d: %w", msDiffCount, err)
		}
		if _, err := fileHandle.Write(msDiffBytes); err != nil {
			return nil, fmt.Errorf("unable to write LS milestone diff #%d: %w", msDiffCount, err)
		}
		increaseOffsets(int64(len(msDiffBytes)), &sepFileOffset)
	}
	timeMilestoneDiffs := time.Now()

	// SEPs
	for {
		sep, err := sepProd()
		if err != nil {
			if errors.Is(err, ErrNoMoreSEPToProduce) {
				break
			}
			return nil, fmt.Errorf("unable to get next LS SEP #%d: %w", sepsCount+1, err)
		}

		sepsCount++
		if _, err := fileHandle.Write(sep[:]); err != nil {
			return nil, fmt.Errorf("unable to write LS SEP #%d: %w", sepsCount, err)
		}
	}
	timeSolidEntryPoints := time.Now()

	// seek back to the file position of the counters
	if _, err := fileHandle.Seek(countersFileOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("unable to seek to LS counter placeholders: %w", err)
	}

	// SEP File Offset
	// The file offset of the SEPs field. This is used to easily update an existing delta snapshot without parsing its content.
	if err := binary.Write(fileHandle, binary.LittleEndian, sepFileOffset); err != nil {
		return nil, fmt.Errorf("unable to write LS solid entry points file offset: %w", err)
	}

	// Milestone Diffs Count
	// The amount of milestone diffs contained within this snapshot.
	if err := binary.Write(fileHandle, binary.LittleEndian, msDiffCount); err != nil {
		return nil, fmt.Errorf("unable to write LS milestone diffs count: %w", err)
	}

	// SEPs Count
	// The amount of SEPs contained within this snapshot.
	if err := binary.Write(fileHandle, binary.LittleEndian, sepsCount); err != nil {
		return nil, fmt.Errorf("unable to write LS solid entry points count: %w", err)
	}

	// update the values in the header
	header.SEPFileOffset = sepFileOffset
	header.MilestoneDiffCount = msDiffCount
	header.SEPCount = sepsCount

	return &SnapshotMetrics{
		DurationHeader:           timeHeader.Sub(timeStart),
		DurationMilestoneDiffs:   timeMilestoneDiffs.Sub(timeHeader),
		DurationSolidEntryPoints: timeSolidEntryPoints.Sub(timeMilestoneDiffs),
	}, nil
}

// ReadSnapshotType reads the snapshot type from the given reader.
func ReadSnapshotType(readSeeker io.ReadSeeker) (Type, error) {
	var version byte
	if err := binary.Read(readSeeker, binary.LittleEndian, &version); err != nil {
		return Full, fmt.Errorf("unable to read LS version: %w", err)
	}

	if version != SupportedFormatVersion {
		return Full, ErrUnsupportedSnapshot
	}

	var snapshotType Type
	if err := binary.Read(readSeeker, binary.LittleEndian, &snapshotType); err != nil {
		return Full, fmt.Errorf("unable to read LS type: %w", err)
	}

	// seek back to the start of the header
	readSeeker.Seek(0, io.SeekStart)

	switch snapshotType {
	case Full:
		return snapshotType, nil
	case Delta:
		return snapshotType, nil
	default:
		return Full, ErrUnsupportedSnapshot
	}
}

// ReadSnapshotHeaderFromFile reads the snapshot type of the given snapshot file.
func ReadSnapshotTypeFromFile(filePath string) (Type, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Full, fmt.Errorf("unable to open snapshot file to read type: %w", err)
	}
	defer func() { _ = file.Close() }()

	return ReadSnapshotType(file)
}

// StreamFullSnapshotDataFrom consumes a full snapshot from the given reader.
func StreamFullSnapshotDataFrom(
	reader io.ReadSeeker,
	protocolStorage *storage.ProtocolStorage,
	headerConsumer FullHeaderConsumerFunc,
	unspentTreasuryOutputConsumer UnspentTreasuryOutputConsumerFunc,
	outputConsumer OutputConsumerFunc,
	msDiffConsumer MilestoneDiffConsumerFunc,
	sepConsumer SEPConsumerFunc) error {

	fullHeader, err := ReadFullSnapshotHeader(reader)
	if err != nil {
		return err
	}

	if err := unspentTreasuryOutputConsumer(fullHeader.TreasuryOutput); err != nil {
		return err
	}

	if err := headerConsumer(fullHeader); err != nil {
		return err
	}

	fullHeaderProtoParams, err := fullHeader.ProtocolParameters()
	if err != nil {
		return err
	}

	// the protocol parameters milestone option in the full snapshot is valid for the ledger milestone index.
	protocolStorage.StoreProtocolParametersMilestoneOption(fullHeader.ProtocolParamsMilestoneOpt)

	for i := uint64(0); i < fullHeader.OutputCount; i++ {
		output, err := ReadOutput(reader, fullHeaderProtoParams)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}

		if err := outputConsumer(output); err != nil {
			return fmt.Errorf("output consumer error at pos %d: %w", i, err)
		}
	}

	// this is the total length of the milestone diffs.
	// we use that to seek back to the start of the diffs after the first iteration, or to seekd to the end in the second one.
	var msDiffsLength int64

	// we need to parse the milestone diffs twice.
	// first round is to get the upcoming protocol parameter changes.
	for i := uint32(0); i < fullHeader.MilestoneDiffCount; i++ {
		msDiffLength, err := ReadMilestoneDiffProtocolParameters(reader, protocolStorage)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}
		increaseOffsets(msDiffLength, &msDiffsLength)
	}
	reader.Seek(-msDiffsLength, io.SeekCurrent)

	// this is the currently parsed length of the milestone diffs.
	// we use that to seek to the end of the milestone diffs.
	var msDiffsParsedLength int64

	// second round is to load the milestone diffs with correct protocol parameters.
	for i := uint32(0); i < fullHeader.MilestoneDiffCount; i++ {
		// the milestone diffs in the full snapshot file are in backwards order.
		msDiffLength, msDiff, err := ReadMilestoneDiff(reader, protocolStorage, false)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}
		increaseOffsets(msDiffLength, &msDiffsParsedLength)

		// we do not consume milestone diffs that are below the target milestone index.
		// these additional milestone diffs are only used to get the protocol parameter updates.
		if msDiff.Milestone.Index < fullHeader.TargetMilestoneIndex {
			// we can break the loop here since we are walking backwards.
			// we also need to jump to the end of the milestone diffs.
			reader.Seek(msDiffsLength-msDiffsParsedLength, io.SeekCurrent)
			break
		}

		if err := msDiffConsumer(msDiff); err != nil {
			return fmt.Errorf("ms-diff consumer error at pos %d: %w", i, err)
		}
	}

	for i := uint16(0); i < fullHeader.SEPCount; i++ {
		solidEntryPointBlockID := iotago.BlockID{}
		if _, err := io.ReadFull(reader, solidEntryPointBlockID[:]); err != nil {
			return fmt.Errorf("unable to read LS SEP at pos %d: %w", i, err)
		}
		if err := sepConsumer(solidEntryPointBlockID, fullHeader.TargetMilestoneIndex); err != nil {
			return fmt.Errorf("SEP consumer error at pos %d: %w", i, err)
		}
	}

	return nil
}

// StreamDeltaSnapshotDataFrom consumes a delta snapshot from the given reader.
// The current milestone index of the protocol manager must be set to the
// target index of the full snapshot file before entering this function.
func StreamDeltaSnapshotDataFrom(
	reader io.ReadSeeker,
	protocolStorage *storage.ProtocolStorage,
	headerConsumer DeltaHeaderConsumerFunc,
	msDiffConsumer MilestoneDiffConsumerFunc,
	sepConsumer SEPConsumerFunc) error {

	deltaHeader, err := ReadDeltaSnapshotHeader(reader)
	if err != nil {
		return err
	}

	if err := headerConsumer(deltaHeader); err != nil {
		return err
	}

	for i := uint32(0); i < deltaHeader.MilestoneDiffCount; i++ {
		_, msDiff, err := ReadMilestoneDiff(reader, protocolStorage, true)
		if err != nil {
			return fmt.Errorf("at pos %d: %w", i, err)
		}

		if err := msDiffConsumer(msDiff); err != nil {
			return fmt.Errorf("ms-diff consumer error at pos %d: %w", i, err)
		}
	}

	for i := uint16(0); i < deltaHeader.SEPCount; i++ {
		solidEntryPointBlockID := iotago.BlockID{}
		if _, err := io.ReadFull(reader, solidEntryPointBlockID[:]); err != nil {
			return fmt.Errorf("unable to read LS SEP at pos %d: %w", i, err)
		}
		if err := sepConsumer(solidEntryPointBlockID, deltaHeader.TargetMilestoneIndex); err != nil {
			return fmt.Errorf("SEP consumer error at pos %d: %w", i, err)
		}
	}

	return nil
}

// reads an Output from the given reader.
func ReadOutput(reader io.ReadSeeker, protoParams *iotago.ProtocolParameters) (*utxo.Output, error) {
	return utxo.OutputFromSnapshotReader(reader, protoParams)
}

// reads a spent from the given reader.
func readSpent(reader io.ReadSeeker, protoParams *iotago.ProtocolParameters, msIndexSpent iotago.MilestoneIndex, msTimestampSpent uint32) (*utxo.Spent, error) {
	return utxo.SpentFromSnapshotReader(reader, protoParams, msIndexSpent, msTimestampSpent)
}

// ReadSnapshotHeaderFromFile reads the header of the given snapshot file.
func ReadSnapshotHeaderFromFile(filePath string, headerConsumer func(readCloser io.ReadCloser) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open snapshot file to read header: %w", err)
	}
	defer func() { _ = file.Close() }()

	return headerConsumer(file)
}

// ReadFullSnapshotHeaderFromFile reads the header of the given full snapshot file.
func ReadFullSnapshotHeaderFromFile(filePath string) (*FullSnapshotHeader, error) {
	var fullSnapshotHeader *FullSnapshotHeader
	if err := ReadSnapshotHeaderFromFile(filePath, func(readCloser io.ReadCloser) error {
		fullHeader, err := ReadFullSnapshotHeader(readCloser)
		if err != nil {
			return err
		}

		fullSnapshotHeader = fullHeader
		return nil
	}); err != nil {
		return nil, err
	}
	return fullSnapshotHeader, nil
}

// ReadDeltaSnapshotHeaderFromFile reads the header of the given delta snapshot file.
func ReadDeltaSnapshotHeaderFromFile(filePath string) (*DeltaSnapshotHeader, error) {
	var deltaSnapshotHeader *DeltaSnapshotHeader
	if err := ReadSnapshotHeaderFromFile(filePath, func(readCloser io.ReadCloser) error {
		deltaHeader, err := ReadDeltaSnapshotHeader(readCloser)
		if err != nil {
			return err
		}

		deltaSnapshotHeader = deltaHeader
		return nil
	}); err != nil {
		return nil, err
	}
	return deltaSnapshotHeader, nil
}
