package gossip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

type ProtocolMsgType byte

const (
	PROTOCOL_VERSION_LEGACY_GOSSIP = 1 << 0
	// STING supports sole transaction-, request-, milestone- and heartbeat messages
	PROTOCOL_VERSION_STING = 1 << 1

	// The amount of bytes dedicated for the message type in the packet header.
	HEADER_TLV_TYPE_BYTES_LENGTH = 1

	// The amount of bytes dedicated for the message length denotation in the packet header.
	HEADER_TLV_LENGTH_BYTES_LENGTH = 2

	// The length of the TLV header
	HEADER_TLV_BYTES_LENGTH = 3

	// The amount of bytes making up the protocol packet header.
	PROTOCOL_HEADER_BYTES_LENGTH = HEADER_TLV_LENGTH_BYTES_LENGTH + HEADER_TLV_TYPE_BYTES_LENGTH

	// The amount of bytes used for the requested transaction hash.
	GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH = 49

	// The amount of bytes used for the requested milestone index.
	GOSSIP_REQUESTED_MS_INDEX_BYTES_LENGTH = 4

	// The amount of bytes used for the a milestone index within a heartbeat packet.
	GOSSIP_HEARTBEAT_MS_INDEX_BYTES_LENGTH = 4

	// The amount of bytes making up the non signature message fragment part of a transaction gossip payload.
	NON_SIG_TX_PART_BYTES_LENGTH = 292

	// The max amount of bytes a signature message fragment is made up from.
	SIG_DATA_MAX_BYTES_LENGTH = 1312

	PROTOCOL_MSG_TYPE_HEADER           ProtocolMsgType = 0
	PROTOCOL_MSG_TYPE_HANDSHAKE        ProtocolMsgType = 1
	PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP ProtocolMsgType = 2
	PROTOCOL_MSG_TYPE_MS_REQUEST       ProtocolMsgType = 3
	PROTOCOL_MSG_TYPE_TX_GOSSIP        ProtocolMsgType = 4
	PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP    ProtocolMsgType = 5
	PROTOCOL_MSG_TYPE_HEARTBEAT        ProtocolMsgType = 6
)

var (
	// Thrown when a packet advertises a message length which is invalid for the given {ProtocolMessage} type.
	ErrInvalidProtocolMessageType   = errors.New("invalid protocol message type")
	ErrInvalidProtocolMessageLength = errors.New("invalid protocol message length")

	// Thrown when an unknown ProtocolMessage type is advertised in a packet.
	ErrUnknownMessageType = errors.New("unknown message type")

	/*
		The supported protocol versions by this node. Bitmasks are used to denote what protocol version this node
		supports in its implementation. The LSB acts as a starting point. Up to 32 bytes are supported in the handshake
		packet, limiting the amount of supported denoted protocol versions to 256.

		Examples:
		[00000001] denotes that this node supports protocol version 1.
		[00000111] denotes that this node supports protocol versions 1, 2 and 3.
		[01101110] denotes that this node supports protocol versions 2, 3, 4, 6 and 7.
		[01101110, 01010001] denotes that this node supports protocol versions 2, 3, 4, 6, 7, 9, 13 and 15.
		[01101110, 01010001, 00010001] denotes that this node supports protocol versions 2, 3, 4, 6, 7, 9, 13, 15, 17 and 21.
	*/

	// supports protocol version(s): 2+1
	SUPPORTED_PROTOCOL_VERSIONS = []byte{PROTOCOL_VERSION_STING | PROTOCOL_VERSION_LEGACY_GOSSIP}

	// The message header sent in each message denoting the TLV fields.
	ProtocolHeaderMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_HEADER, MaxLength: PROTOCOL_HEADER_BYTES_LENGTH, SupportsDynamicLength: false}

	// The initial handshake packet sent over the wire up on a new neighbor connection.
	// Made up of:
	// - own server socket port (2 bytes)
	// - time at which the packet was sent (8 bytes)
	// - own used byte encoded coordinator address (49 bytes)
	// - own used MWM (1 byte)
	// - supported protocol versions. we need up to 32 bytes to represent 256 possible protocol
	//   versions. only up to N bytes are used to communicate the highest supported version.
	ProtocolHandshakeMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_HANDSHAKE, MaxLength: 92, SupportsDynamicLength: true}

	// The transaction payload + requested transaction hash gossipping packet. In reality most of this packets won't
	// take up their full 1604 bytes as the signature message fragment of the tx is truncated.
	ProtocolLegacyTransactionGossipMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP, MaxLength: GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH + NON_SIG_TX_PART_BYTES_LENGTH + SIG_DATA_MAX_BYTES_LENGTH, SupportsDynamicLength: true}

	// The transaction gossipping packet. Contains only a tx payload.
	ProtocolTransactionGossipMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_TX_GOSSIP, MaxLength: NON_SIG_TX_PART_BYTES_LENGTH + SIG_DATA_MAX_BYTES_LENGTH, SupportsDynamicLength: true}

	// The requested transaction hash gossipping packet. Contains only a hash of a requested transaction payload.
	ProtocolTransactionRequestGossipMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP, MaxLength: GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH, SupportsDynamicLength: false}

	// The heartbeat packet containing the current latest solid and pruned milestone index.
	ProtocolHeartbeatMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_HEARTBEAT, MaxLength: GOSSIP_HEARTBEAT_MS_INDEX_BYTES_LENGTH * 2, SupportsDynamicLength: false}

	// The requested milestone index packet.
	ProtocolMilestoneRequestMsg = ProtocolMessage{TypeID: PROTOCOL_MSG_TYPE_MS_REQUEST, MaxLength: GOSSIP_REQUESTED_MS_INDEX_BYTES_LENGTH, SupportsDynamicLength: false}
)

type ProtocolMessage struct {
	TypeID                ProtocolMsgType
	MaxLength             uint16
	SupportsDynamicLength bool
}

// Gets the ProtocolMessage corresponding to the given type id.
func GetProtocolMsgFromTypeID(typeID ProtocolMsgType) (*ProtocolMessage, error) {

	switch typeID {
	case PROTOCOL_MSG_TYPE_HEADER:
		deepCopy := ProtocolHeaderMsg
		return &deepCopy, nil

	case PROTOCOL_MSG_TYPE_HANDSHAKE:
		deepCopy := ProtocolHandshakeMsg
		return &deepCopy, nil

	case PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP:
		deepCopy := ProtocolLegacyTransactionGossipMsg
		return &deepCopy, nil

	case PROTOCOL_MSG_TYPE_MS_REQUEST:
		deepCopy := ProtocolMilestoneRequestMsg
		return &deepCopy, nil

	case PROTOCOL_MSG_TYPE_TX_GOSSIP:
		deepCopy := ProtocolTransactionGossipMsg
		return &deepCopy, nil

	case PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP:
		deepCopy := ProtocolTransactionRequestGossipMsg
		return &deepCopy, nil

	case PROTOCOL_MSG_TYPE_HEARTBEAT:
		deepCopy := ProtocolHeartbeatMsg
		return &deepCopy, nil

	default:
		return nil, fmt.Errorf("%s: %d", ErrInvalidProtocolMessageType, typeID)
	}
}

// The ProtocolHeader denotes the protocol version used by the node and the TLV of the packet.
type ProtocolHeader struct {
	MsgType       ProtocolMsgType
	ProtoMsg      *ProtocolMessage
	MessageLength uint16
}

// Parses the given buffer into a ProtocolHeader.
// The IRI protocol uses a 4 bytes header denoting the version, type and length of a packet.
func ParseHeader(buf []byte) (*ProtocolHeader, error) {

	// extract type of message
	msgType := ProtocolMsgType(buf[0])

	protoMsg, err := GetProtocolMsgFromTypeID(msgType)
	if err != nil {
		return nil, err
	}

	// extract length of message
	advertisedMsgLength := binary.BigEndian.Uint16(buf[1:3])

	if (advertisedMsgLength > protoMsg.MaxLength) || (!protoMsg.SupportsDynamicLength && (advertisedMsgLength < protoMsg.MaxLength)) {
		return nil, fmt.Errorf("%s: advertised length: %d bytes; max length: %d bytes", ErrInvalidProtocolMessageLength.Error(), advertisedMsgLength, protoMsg.MaxLength)
	}

	return &ProtocolHeader{MsgType: msgType, ProtoMsg: protoMsg, MessageLength: advertisedMsgLength}, nil
}

// Adds the protocol header to the given ByteBuffer.
func AddProtocolHeader(buf *bytes.Buffer, protoMsgType ProtocolMsgType, payloadLengthBytes uint16) error {
	err := binary.Write(buf, binary.BigEndian, byte(protoMsgType))
	if err != nil {
		return err
	}
	return binary.Write(buf, binary.BigEndian, payloadLengthBytes)
}

// Creates a new transaction and request gossip packet.
//	The transaction to add into the packet
//	requestedHash The hash of the requested transaction
//	return a {@link ByteBuffer} containing the transaction gossip packet.
func CreateLegacyTransactionGossipPacket(truncatedTxData []byte, requestedHash []byte) ([]byte, error) {

	payloadLengthBytes := uint16(len(truncatedTxData) + GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH)

	buf := bytes.NewBuffer(make([]byte, 0, ProtocolHeaderMsg.MaxLength+payloadLengthBytes))

	err := AddProtocolHeader(buf, PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP, payloadLengthBytes)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, truncatedTxData)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, requestedHash[:GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH])
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Creates a new transaction gossip packet.
//	The transaction to add into the packet
//	return a {@link ByteBuffer} containing the transaction gossip packet.
func CreateTransactionGossipPacket(truncatedTxData []byte) ([]byte, error) {

	payloadLengthBytes := uint16(len(truncatedTxData))
	buf := bytes.NewBuffer(make([]byte, 0, ProtocolHeaderMsg.MaxLength+payloadLengthBytes))

	err := AddProtocolHeader(buf, PROTOCOL_MSG_TYPE_TX_GOSSIP, payloadLengthBytes)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, truncatedTxData)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Creates a transaction request gossip packet.
//	requestedHash The hash of the requested transaction
//	return a {@link ByteBuffer} containing the transaction gossip packet.
func CreateTransactionRequestGossipPacket(requestedHash []byte) ([]byte, error) {

	buf := bytes.NewBuffer(make([]byte, 0, ProtocolHeaderMsg.MaxLength+GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH))
	err := AddProtocolHeader(buf, PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP, GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, requestedHash[:GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH])
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Creates a new heartbeat packet.
func CreateHeartbeatPacket(solidMilestoneIndex milestone_index.MilestoneIndex, prunedMilestoneIndex milestone_index.MilestoneIndex) ([]byte, error) {

	buf := bytes.NewBuffer(make([]byte, 0, ProtocolHeaderMsg.MaxLength+GOSSIP_HEARTBEAT_MS_INDEX_BYTES_LENGTH*2))
	err := AddProtocolHeader(buf, PROTOCOL_MSG_TYPE_HEARTBEAT, GOSSIP_HEARTBEAT_MS_INDEX_BYTES_LENGTH*2)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, solidMilestoneIndex)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, prunedMilestoneIndex)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Creates a new milestone request packet.
//	requestedMilestoneIndex The index of the requested milestone
//	return a {@link ByteBuffer} containing the milestone request packet.
func CreateMilestoneRequestPacket(requestedMilestoneIndex milestone_index.MilestoneIndex) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, ProtocolHeaderMsg.MaxLength+GOSSIP_REQUESTED_MS_INDEX_BYTES_LENGTH))
	err := AddProtocolHeader(buf, PROTOCOL_MSG_TYPE_MS_REQUEST, GOSSIP_REQUESTED_MS_INDEX_BYTES_LENGTH)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, requestedMilestoneIndex)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Copies the requested transaction hash from the given source data byte array into the given destination byte array.
//	source the transaction gossip packet data
func ExtractRequestedTxHash(source []byte) []byte {
	reqHashBytes := make([]byte, GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH)
	copy(reqHashBytes, source[len(source)-GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH:len(source)])
	return reqHashBytes
}

// Copies the requested transaction hash from the given source data byte array into the given destination byte array.
//	source the transaction gossip packet data
func ExtractRequestedMilestoneIndex(source []byte) (milestone_index.MilestoneIndex, error) {
	if len(source) != 4 {
		return 0, ErrInvalidProtocolMessageLength
	}

	msIndex := binary.BigEndian.Uint32(source)
	return milestone_index.MilestoneIndex(msIndex), nil
}
