package sting

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/gohornet/hornet/pkg/consts"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/protocol/message"
	"github.com/gohornet/hornet/pkg/protocol/tlv"
)

var (
	// ErrInvalidSourceLength is returned when an invalid source byte slice for extraction of certain data is passed.
	ErrInvalidSourceLength = errors.New("invalid source byte slice")
)

// FeatureSet denotes the version bit for STING support.
const FeatureSet = 1 << 1

// FeatureSetName is the name of the STING feature set.
const FeatureSetName = "STING"

func init() {
	if err := message.RegisterType(MessageTypeMilestoneRequest, MilestoneRequestMessageDefinition); err != nil {
		panic(err)
	}
	if err := message.RegisterType(MessageTypeTransaction, TransactionMessageDefinition); err != nil {
		panic(err)
	}
	if err := message.RegisterType(MessageTypeTransactionRequest, TransactionRequestMessageDefinition); err != nil {
		panic(err)
	}
	if err := message.RegisterType(MessageTypeHeartbeat, HeartbeatMessageDefinition); err != nil {
		panic(err)
	}
}

const (
	MessageTypeMilestoneRequest   message.Type = 3
	MessageTypeTransaction        message.Type = 4
	MessageTypeTransactionRequest message.Type = 5
	MessageTypeHeartbeat          message.Type = 6
)

const (
	// The amount of bytes used for the requested transaction hash.
	RequestedTransactionHashMsgBytesLength = 49

	// The amount of bytes used for the requested milestone index.
	RequestedMilestoneIndexMsgBytesLength = 4

	// The amount of bytes used for a milestone index within a heartbeat packet.
	HeartbeatMilestoneIndexBytesLength = 4

	// The index to use to request the latest milestone via a milestone request message.
	LatestMilestoneRequestIndex = 0
)

var (
	// TransactionMessageFormat defines a transaction message's format.
	TransactionMessageDefinition = &message.Definition{
		ID:             MessageTypeTransaction,
		MaxBytesLength: consts.NonSigTxPartBytesLength + consts.SigDataMaxBytesLength,
		VariableLength: true,
	}

	// The requested transaction hash gossipping packet.
	// Contains only a hash of a requested transaction payload.
	TransactionRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeTransactionRequest,
		MaxBytesLength: RequestedTransactionHashMsgBytesLength,
		VariableLength: false,
	}

	// The heartbeat packet containing the current latest solid and pruned milestone index.
	HeartbeatMessageDefinition = &message.Definition{
		ID:             MessageTypeHeartbeat,
		MaxBytesLength: HeartbeatMilestoneIndexBytesLength * 2,
		VariableLength: false,
	}

	// The requested milestone index packet.
	MilestoneRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeMilestoneRequest,
		MaxBytesLength: RequestedMilestoneIndexMsgBytesLength,
		VariableLength: false,
	}
)

// NewTransactionMessage creates a new transaction message.
func NewTransactionMessage(txData []byte) ([]byte, error) {
	msgBytesLength := uint16(len(txData))
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+msgBytesLength))

	if err := tlv.WriteHeader(buf, MessageTypeTransaction, msgBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, txData); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewTransactionRequestMessage creates a transaction request message.
func NewTransactionRequestMessage(requestedHash hornet.Hash) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+RequestedTransactionHashMsgBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeTransactionRequest, RequestedTransactionHashMsgBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, requestedHash[:RequestedTransactionHashMsgBytesLength]); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewHeartbeatMessage creates a new heartbeat message.
func NewHeartbeatMessage(solidMilestoneIndex milestone.Index, prunedMilestoneIndex milestone.Index) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+HeartbeatMilestoneIndexBytesLength*2))
	if err := tlv.WriteHeader(buf, MessageTypeHeartbeat, HeartbeatMilestoneIndexBytesLength*2); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, solidMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, prunedMilestoneIndex); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewMilestoneRequestMessage creates a new milestone request message.
func NewMilestoneRequestMessage(requestedMilestoneIndex milestone.Index) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+RequestedMilestoneIndexMsgBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeMilestoneRequest, RequestedMilestoneIndexMsgBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, requestedMilestoneIndex); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ExtractRequestedTransactionHash extracts the requested transaction hash from the given source.
func ExtractRequestedTransactionHash(source []byte) []byte {
	reqHashBytes := make([]byte, RequestedTransactionHashMsgBytesLength)
	copy(reqHashBytes, source[len(source)-RequestedTransactionHashMsgBytesLength:])
	return reqHashBytes
}

// ExtractRequestedMilestoneIndex extracts the requested milestone index from the given source.
func ExtractRequestedMilestoneIndex(source []byte) (milestone.Index, error) {
	if len(source) != 4 {
		return 0, ErrInvalidSourceLength
	}

	msIndex := binary.BigEndian.Uint32(source)
	return milestone.Index(msIndex), nil
}
