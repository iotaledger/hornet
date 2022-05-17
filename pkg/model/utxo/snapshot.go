package utxo

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// Helpers to serialize/deserialize into/from snapshots

func (o *Output) SnapshotBytes() []byte {
	m := marshalutil.New()
	m.WriteBytes(o.outputID[:])
	m.WriteBytes(o.blockID)
	m.WriteUint32(uint32(o.milestoneIndex))
	m.WriteUint32(o.milestoneTimestamp)

	bytes, err := o.output.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		panic(err)
	}
	m.WriteBytes(bytes)

	return m.Bytes()
}

func OutputFromSnapshotReader(reader io.ReadSeeker, protoParas *iotago.ProtocolParameters) (*Output, error) {
	outputID := iotago.OutputID{}
	if _, err := io.ReadFull(reader, outputID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS output ID: %w", err)
	}

	blockID := iotago.BlockID{}
	if _, err := io.ReadFull(reader, blockID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS block ID: %w", err)
	}

	var confirmationIndex uint32
	if err := binary.Read(reader, binary.LittleEndian, &confirmationIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS output milestone index: %w", err)
	}

	var milestoneTimestamp uint32
	if err := binary.Read(reader, binary.LittleEndian, &milestoneTimestamp); err != nil {
		return nil, fmt.Errorf("unable to read LS output milestone timestamp: %w", err)
	}

	buffer := make([]byte, iotago.BlockBinSerializedMaxSize)
	bufferLen, err := reader.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("unable to read LS output bytes: %w", err)
	}

	if bufferLen == 0 {
		return nil, fmt.Errorf("unable to read LS output: buffer length: %d", bufferLen)
	}

	output, err := iotago.OutputSelector(uint32(buffer[0]))
	if err != nil {
		return nil, fmt.Errorf("unable to determine output type of LS output: %w", err)
	}

	outputLen, err := output.Deserialize(buffer, serializer.DeSeriModePerformValidation, protoParas)
	if err != nil {
		return nil, fmt.Errorf("invalid LS output address: %w", err)
	}

	// Seek back the bytes we did not consume during serialization
	_, err = reader.Seek(int64(-bufferLen+outputLen), io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("invalid LS output length: %w", err)
	}

	return CreateOutput(&outputID, hornet.BlockIDFromArray(blockID), milestone.Index(confirmationIndex), milestoneTimestamp, output), nil
}

func (s *Spent) SnapshotBytes() []byte {
	m := marshalutil.New()
	m.WriteBytes(s.Output().SnapshotBytes())
	m.WriteBytes(s.targetTransactionID[:])
	return m.Bytes()
}

func SpentFromSnapshotReader(reader io.ReadSeeker, protoParas *iotago.ProtocolParameters, msIndex milestone.Index, msTimestamp uint32) (*Spent, error) {
	output, err := OutputFromSnapshotReader(reader, protoParas)
	if err != nil {
		return nil, err
	}

	transactionID := &iotago.TransactionID{}
	if _, err := io.ReadFull(reader, transactionID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS target transaction ID: %w", err)
	}

	return NewSpent(output, transactionID, msIndex, msTimestamp), nil
}
