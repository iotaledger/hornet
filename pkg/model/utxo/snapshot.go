package utxo

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// Helpers to serialize/deserialize into/from snapshots

func (o *Output) SnapshotBytes() []byte {
	m := marshalutil.New()
	m.WriteBytes(o.outputID[:])
	m.WriteBytes(o.blockID[:])
	m.WriteUint32(o.msIndexBooked)
	m.WriteUint32(o.msTimestampBooked)
	m.WriteUint32(uint32(len(o.outputData)))
	m.WriteBytes(o.outputData)

	return m.Bytes()
}

func OutputFromSnapshotReader(reader io.ReadSeeker, protoParams *iotago.ProtocolParameters) (*Output, error) {
	outputID := iotago.OutputID{}
	if _, err := io.ReadFull(reader, outputID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS output ID: %w", err)
	}

	blockID := iotago.BlockID{}
	if _, err := io.ReadFull(reader, blockID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS block ID: %w", err)
	}

	var msIndexBooked iotago.MilestoneIndex
	if err := binary.Read(reader, binary.LittleEndian, &msIndexBooked); err != nil {
		return nil, fmt.Errorf("unable to read LS output milestone index booked: %w", err)
	}

	var msTimestampBooked uint32
	if err := binary.Read(reader, binary.LittleEndian, &msTimestampBooked); err != nil {
		return nil, fmt.Errorf("unable to read LS output milestone timestamp booked: %w", err)
	}

	var outputLength uint32
	if err := binary.Read(reader, binary.LittleEndian, &outputLength); err != nil {
		return nil, fmt.Errorf("unable to read LS output length: %w", err)
	}

	outputBytes := make([]byte, outputLength)
	if _, err := io.ReadFull(reader, outputBytes); err != nil {
		return nil, fmt.Errorf("unable to read LS output bytes: %w", err)
	}

	output, err := iotago.OutputSelector(uint32(outputBytes[0]))
	if err != nil {
		return nil, fmt.Errorf("unable to determine output type of LS output: %w", err)
	}

	if _, err := output.Deserialize(outputBytes, serializer.DeSeriModePerformValidation, protoParams); err != nil {
		return nil, fmt.Errorf("invalid LS output address: %w", err)
	}

	return CreateOutput(outputID, blockID, msIndexBooked, msTimestampBooked, output), nil
}

func (s *Spent) SnapshotBytes() []byte {
	m := marshalutil.New()
	m.WriteBytes(s.Output().SnapshotBytes())
	m.WriteBytes(s.transactionIDSpent[:])
	// we don't need to write msIndexSpent and msTimestampSpent because this info is available in the milestoneDiff that consumes the output
	return m.Bytes()
}

func SpentFromSnapshotReader(reader io.ReadSeeker, protoParams *iotago.ProtocolParameters, msIndexSpent iotago.MilestoneIndex, msTimestampSpent uint32) (*Spent, error) {
	output, err := OutputFromSnapshotReader(reader, protoParams)
	if err != nil {
		return nil, err
	}

	transactionIDSpent := iotago.TransactionID{}
	if _, err := io.ReadFull(reader, transactionIDSpent[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS transaction ID spent: %w", err)
	}

	return NewSpent(output, transactionIDSpent, msIndexSpent, msTimestampSpent), nil
}
