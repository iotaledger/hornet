package handshake

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/willf/bitset"

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
func (hs Handshake) SupportedVersion(ownSupportedMessagesBitset *bitset.BitSet) (version int, err error) {
	hsSupportedMessagesBitset := bitset.New(uint(len(hs.SupportedVersions) * 8))
	hsSupportedMessagesBitset.UnmarshalBinary(hs.SupportedVersions)

	bothSupportedMessagesBitset := hsSupportedMessagesBitset.Union(ownSupportedMessagesBitset)

	if !bothSupportedMessagesBitset.Any() {
		// we don't support any protocol version the peer supports
		// return the highest supported version of a given node
		for i := int(hsSupportedMessagesBitset.Len()) - 1; i >= 0; i-- {
			if hsSupportedMessagesBitset.Test(uint(i)) {
				return 1 << i, ErrVersionNotSupported
			}
		}

	}

	for i := int(bothSupportedMessagesBitset.Len()) - 1; i >= 0; i-- {
		if bothSupportedMessagesBitset.Test(uint(i)) {
			return 1 << i, nil
		}
	}

	return 0, ErrVersionNotSupported
}

// NewHandshakeMessage creates a new handshake message.
func NewHandshakeMessage(ownSupportedMessagesBitset *bitset.BitSet, ownSourcePort uint16, ownByteEncodedCooAddress []byte, ownUsedMWM byte) ([]byte, error) {

	maxLength := HandshakeMessageDefinition.MaxBytesLength

	supportedMessageTypes, err := ownSupportedMessagesBitset.MarshalBinary()
	if err != nil {
		return nil, err
	}

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
