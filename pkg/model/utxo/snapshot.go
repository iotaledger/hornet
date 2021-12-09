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

	bytes, err := o.output.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		panic(err)
	}
	m.WriteUint32(uint32(len(bytes)))
	m.WriteBytes(bytes)

	return m.Bytes()
}

func OutputFromSnapshotReader(reader io.Reader, deSeriParas *iotago.DeSerializationParameters) (*Output, error) {
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

	var outputLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &outputLen); err != nil {
		return nil, fmt.Errorf("unable to read LS output length: %w", err)
	}

	outputBytes := make([]byte, outputLen)
	if _, err := io.ReadFull(reader, outputBytes); err != nil {
		return nil, fmt.Errorf("unable to read LS output bytes: %w", err)
	}

	output, err := iotago.OutputSelector(uint32(outputBytes[0]))
	if err != nil {
		return nil, fmt.Errorf("unable to determine output type of LS output: %w", err)
	}

	if _, err := output.Deserialize(outputBytes, serializer.DeSeriModePerformValidation, deSeriParas); err != nil {
		return nil, fmt.Errorf("invalid LS output address: %w", err)
	}

	return CreateOutput(&outputID, hornet.MessageIDFromArray(messageID), milestone.Index(confirmationIndex), uint64(milestoneTimestamp), output), nil
}

func (s *Spent) SnapshotBytes() []byte {
	m := marshalutil.New()
	m.WriteBytes(s.Output().SnapshotBytes())
	m.WriteBytes(s.targetTransactionID[:])
	return m.Bytes()
}

func SpentFromSnapshotReader(reader io.Reader, deSeriParas *iotago.DeSerializationParameters, index milestone.Index) (*Spent, error) {
	output, err := OutputFromSnapshotReader(reader, deSeriParas)
	if err != nil {
		return nil, err
	}

	transactionID := &iotago.TransactionID{}
	if _, err := io.ReadFull(reader, transactionID[:]); err != nil {
		return nil, fmt.Errorf("unable to read LS target transaction ID: %w", err)
	}

	return NewSpent(output, transactionID, index), nil
}
