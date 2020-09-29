package sting

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/protocol/message"
	"github.com/gohornet/hornet/pkg/protocol/tlv"
)

var (
	// ErrInvalidSourceLength is returned when an invalid source byte slice for extraction of certain data is passed.
	ErrInvalidSourceLength = errors.New("invalid source byte slice")
)

// MinimumVersion denotes the minimum version for Chrysalis-Pt2 support.
const MinimumVersion = 1

// FeatureSetName is the name of the feature set.
const FeatureSetName = "Chrysalis-Pt2"

func init() {
	if err := message.RegisterType(MessageTypeMilestoneRequest, MilestoneRequestMessageDefinition); err != nil {
		panic(err)
	}
	if err := message.RegisterType(MessageTypeMessage, MessageMessageDefinition); err != nil {
		panic(err)
	}
	if err := message.RegisterType(MessageTypeMessageRequest, MessageRequestMessageDefinition); err != nil {
		panic(err)
	}
	if err := message.RegisterType(MessageTypeHeartbeat, HeartbeatMessageDefinition); err != nil {
		panic(err)
	}
}

const (
	MessageTypeMilestoneRequest message.Type = 3
	MessageTypeMessage          message.Type = 4
	MessageTypeMessageRequest   message.Type = 5
	MessageTypeHeartbeat        message.Type = 6
)

const (
	// The amount of bytes used for the requested message ID.
	RequestedMessageIDMsgBytesLength = 32

	// The amount of bytes used for the requested milestone index.
	RequestedMilestoneIndexMsgBytesLength = 4

	// The amount of bytes used for a milestone index within a heartbeat packet.
	HeartbeatMilestoneIndexBytesLength = 4

	// The index to use to request the latest milestone via a milestone request message.
	LatestMilestoneRequestIndex = 0
)

var (
	// MessageMessageDefinition defines a message message's format.
	MessageMessageDefinition = &message.Definition{
		ID:             MessageTypeMessage,
		MaxBytesLength: 1024, // ToDo
		VariableLength: true,
	}

	// The requested message ID gossipping packet.
	// Contains only an ID of a requested message payload.
	MessageRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeMessageRequest,
		MaxBytesLength: RequestedMessageIDMsgBytesLength,
		VariableLength: false,
	}

	// The heartbeat packet containing the current latest solid, pruned and latest milestone index,
	// number of connected neighbors and number of synced neighbors.
	HeartbeatMessageDefinition = &message.Definition{
		ID:             MessageTypeHeartbeat,
		MaxBytesLength: HeartbeatMilestoneIndexBytesLength*3 + 2,
		VariableLength: false,
	}

	// The requested milestone index packet.
	MilestoneRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeMilestoneRequest,
		MaxBytesLength: RequestedMilestoneIndexMsgBytesLength,
		VariableLength: false,
	}
)

// NewMessageMsg creates a new message message.
func NewMessageMsg(msgData []byte) ([]byte, error) {
	msgBytesLength := uint16(len(msgData))
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+msgBytesLength))

	if err := tlv.WriteHeader(buf, MessageTypeMessage, msgBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, msgData); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewMessageRequestMsg creates a message request message.
func NewMessageRequestMsg(requestedMessageID hornet.Hash) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+MessageRequestMessageDefinition.MaxBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeMessageRequest, MessageRequestMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, requestedMessageID[:RequestedMessageIDMsgBytesLength]); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewHeartbeatMsg creates a new heartbeat message.
func NewHeartbeatMsg(solidMilestoneIndex milestone.Index, prunedMilestoneIndex milestone.Index, latestMilestoneIndex milestone.Index, connectedNeighbors uint8, syncedNeighbors uint8) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+HeartbeatMessageDefinition.MaxBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeHeartbeat, HeartbeatMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, solidMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, prunedMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, latestMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, connectedNeighbors); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, syncedNeighbors); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewMilestoneRequestMsg creates a new milestone request message.
func NewMilestoneRequestMsg(requestedMilestoneIndex milestone.Index) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+MilestoneRequestMessageDefinition.MaxBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeMilestoneRequest, MilestoneRequestMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, requestedMilestoneIndex); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ExtractRequestedMilestoneIndex extracts the requested milestone index from the given source.
func ExtractRequestedMilestoneIndex(source []byte) (milestone.Index, error) {
	if len(source) != 4 {
		return 0, ErrInvalidSourceLength
	}

	msIndex := binary.LittleEndian.Uint32(source)
	return milestone.Index(msIndex), nil
}
