package handshake

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/gohornet/hornet/pkg/protocol/message"
	"github.com/gohornet/hornet/pkg/protocol/tlv"
)

func init() {
	if err := message.RegisterType(MessageTypeHandshake, HandshakeMessageDefinition); err != nil {
		panic(err)
	}
}

const (
	MessageTypeHandshake message.Type = 1
)

var (
	// HandshakeMessageFormat defines a handshake message's format.
	// Made up of:
	// - own server socket port (2 bytes)
	// - time at which the packet was sent (8 bytes)
	// - own used byte encoded coordinator address (49 bytes)
	// - own used MWM (1 byte)
	// - supported protocol versions. we need up to 32 bytes to represent 256 possible protocol
	//   versions. only up to N bytes are used to communicate the highest supported version.
	HandshakeMessageDefinition = &message.Definition{
		ID:             MessageTypeHandshake,
		MaxBytesLength: 92,
		VariableLength: true,
	}
)

type HeaderState int32

const (
	// The amount of bytes used for the coo address sent in a handshake packet.
	ByteEncodedCooAddressBytesLength = 49
)

var (
	ErrVersionNotSupported = errors.New("version not supported")
)

// Handshake defines information exchanged during the handshake phase between two peers.
type Handshake struct {
	ServerSocketPort      uint16
	SentTimestamp         uint64
	ByteEncodedCooAddress []byte
	MWM                   byte
	SupportedVersions     []byte
}

// SupportedVersion returns the highest supported protocol version.
func (hs Handshake) SupportedVersion(supportedMessageTypes []byte) (version int, err error) {
	highestSupportedVersion := 0
	highestSupportedVersionByPeer := 0

	for i, ownSupportedVersion := range supportedMessageTypes {
		// max check up to advertised versions by the peer
		if i >= len(hs.SupportedVersions) {
			break
		}

		// get versions matched by both
		supported := hs.SupportedVersions[i] & ownSupportedVersion

		// none supported
		if supported == 0 {
			continue
		}

		// iterate through all bits and find highest (more to the left is higher)
		highest := 0
		var j uint
		for j = 0; j < 8; j++ {
			if ((supported >> j) & 1) == 1 {
				highest = int(j + 1)
			}
		}
		highestSupportedVersion = highest + (i * 8)
	}

	// if the highest version is still 0, it means that we don't support
	// any protocol version the peer supports
	if highestSupportedVersion == 0 {
		// grab last byte denoting the highest versions.
		// a node will only hold version bytes if at least one version in that
		// byte is supported, therefore it's safe to assume, that the last byte contains
		// the highest supported version of a given node.
		lastVersionsByte := hs.SupportedVersions[len(hs.SupportedVersions)-1]

		// iterate through all bits and find highest (more to the left is higher)
		highest := 0
		var j uint
		for j = 0; j < 8; j++ {
			if ((lastVersionsByte >> j) & 0x01) == 0x01 {
				highest = int(j + 1)
			}
		}

		highestSupportedVersionByPeer = highest + ((len(hs.SupportedVersions) - 1) * 8)

		return highestSupportedVersionByPeer, ErrVersionNotSupported
	}

	return highestSupportedVersion, nil
}

// NewHandshakeMessage creates a new handshake message.
func NewHandshakeMessage(supportedMessageTypes []byte, ownSourcePort uint16, ownByteEncodedCooAddress []byte, ownUsedMWM byte) ([]byte, error) {

	maxLength := HandshakeMessageDefinition.MaxBytesLength
	payloadLengthBytes := maxLength - (maxLength - 60) + uint16(len(supportedMessageTypes))
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+payloadLengthBytes))

	if err := tlv.WriteHeader(buf, MessageTypeHandshake, payloadLengthBytes); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, ownSourcePort); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, time.Now().UnixNano()/int64(time.Millisecond)); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, ownByteEncodedCooAddress); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, ownUsedMWM); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, supportedMessageTypes); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ParseHandshake parses the given message into a Handshake.
func ParseHandshake(msg []byte) (*Handshake, error) {
	var serverSocketPort uint16
	var sentTimestamp uint64
	byteEncodedCooAddress := make([]byte, ByteEncodedCooAddressBytesLength)
	var mwm byte
	supportedVersions := make([]byte, 8)

	r := bytes.NewReader(msg)

	if err := binary.Read(r, binary.BigEndian, &serverSocketPort); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &sentTimestamp); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &byteEncodedCooAddress); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &mwm); err != nil {
		return nil, err
	}

	if _, err := r.Read(supportedVersions); err != nil {
		return nil, err
	}

	hs := &Handshake{ServerSocketPort: serverSocketPort, SentTimestamp: sentTimestamp, ByteEncodedCooAddress: byteEncodedCooAddress, MWM: mwm, SupportedVersions: supportedVersions}
	return hs, nil
}
