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
	m.WriteBytes(o.messageID)
	m.WriteBytes(o.outputID[:])
	m.WriteUint32(uint32(o.milestoneIndex))
	m.WriteUint32(o.milestoneTimestamp)

	bytes, err := o.output.Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		panic(err)
	}
	m.WriteBytes(bytes)

	return m.Bytes()
}

func OutputFromSnapshotReader(reader io.ReadSeeker, deSeriParas *iotago.DeSerializationParameters) (*Output, error) {
	messageID := iotago.MessageID{}
	if _, err := io.ReadFull(reader, messageID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS message ID: %w", err)
	}

	outputID := iotago.OutputID{}
	if _, err := io.ReadFull(reader, outputID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS output ID: %w", err)
	}

	var confirmationIndex uint32
	if err := binary.Read(reader, binary.LittleEndian, &confirmationIndex); err != nil {
		return nil, fmt.Errorf("unable to read LS output milestone index: %w", err)
	}

	var milestoneTimestamp uint32
	if err := binary.Read(reader, binary.LittleEndian, &milestoneTimestamp); err != nil {
		return nil, fmt.Errorf("unable to read LS output milestone timestamp: %w", err)
	}

	buffer := make([]byte, iotago.MessageBinSerializedMaxSize)
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

	outputLen, err := output.Deserialize(buffer, serializer.DeSeriModePerformValidation, deSeriParas)
	if err != nil {
		return nil, fmt.Errorf("invalid LS output address: %w", err)
	}

	// Seek back the bytes we did not consume during serialization
	_, err = reader.Seek(int64(-bufferLen+outputLen), io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("invalid LS output length: %w", err)
	}

	return CreateOutput(&outputID, hornet.MessageIDFromArray(messageID), milestone.Index(confirmationIndex), uint64(milestoneTimestamp), output), nil
}

func (s *Spent) SnapshotBytes() []byte {
	m := marshalutil.New()
	m.WriteBytes(s.Output().SnapshotBytes())
	m.WriteBytes(s.targetTransactionID[:])
	return m.Bytes()
}

func SpentFromSnapshotReader(reader io.ReadSeeker, deSeriParas *iotago.DeSerializationParameters, msIndex milestone.Index, msTimestamp uint64) (*Spent, error) {
	output, err := OutputFromSnapshotReader(reader, deSeriParas)
	if err != nil {
		return nil, err
	}

	transactionID := &iotago.TransactionID{}
	if _, err := io.ReadFull(reader, transactionID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS target transaction ID: %w", err)
	}

	return NewSpent(output, transactionID, msIndex, msTimestamp), nil
}
