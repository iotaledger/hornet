package handshake

import (
	"bytes"
	"crypto/ed25519"
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
	// - own used coordinator public key (32 bytes)
	// - own used MWM (1 byte)
	// - version (2 byte)
	HandshakeMessageDefinition = &message.Definition{
		ID:             MessageTypeHandshake,
		MaxBytesLength: 2 + 8 + ed25519.PublicKeySize + 1 + 2,
		VariableLength: false,
	}
)

type HeaderState int32

var (
	ErrVersionNotSupported = errors.New("version not supported")
)

// Handshake defines information exchanged during the handshake phase between two peers.
type Handshake struct {
	ServerSocketPort uint16
	SentTimestamp    uint64
	CooPublicKey     ed25519.PublicKey
	MWM              byte
	Version          uint16
}

// VersionSupported returns if the protocol version is supported by this node.
func (hs Handshake) VersionSupported(ownMinimumVersion uint16) (version uint16, err error) {
	if hs.Version < ownMinimumVersion {
		return hs.Version, ErrVersionNotSupported
	}

	return hs.Version, nil
}

// NewHandshakeMsg creates a new handshake message.
func NewHandshakeMsg(ownVersion uint16, ownSourcePort uint16, ownCooPublicKey ed25519.PublicKey, ownUsedMWM byte) ([]byte, error) {

	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+HandshakeMessageDefinition.MaxBytesLength))

	if err := tlv.WriteHeader(buf, MessageTypeHandshake, HandshakeMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, ownSourcePort); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, time.Now().UnixNano()/int64(time.Millisecond)); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, ownCooPublicKey); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, ownUsedMWM); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, ownVersion); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ParseHandshake parses the given message into a Handshake.
func ParseHandshake(msg []byte) (*Handshake, error) {
	var serverSocketPort uint16
	var sentTimestamp uint64
	var cooPublicKey = make([]byte, ed25519.PublicKeySize)
	var mwm byte
	var version uint16

	r := bytes.NewReader(msg)

	if err := binary.Read(r, binary.LittleEndian, &serverSocketPort); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &sentTimestamp); err != nil {
		return nil, err
	}

	if _, err := r.Read(cooPublicKey); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &mwm); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, err
	}

	hs := &Handshake{ServerSocketPort: serverSocketPort, SentTimestamp: sentTimestamp, CooPublicKey: cooPublicKey, MWM: mwm, Version: version}
	return hs, nil
}
