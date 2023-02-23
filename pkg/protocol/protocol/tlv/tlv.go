package tlv

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol/message"
)

var (
	// ErrInvalidMessageLength is returned when a packet advertises a message length which
	// is invalid for the given message type.
	ErrInvalidMessageLength = errors.New("invalid message length")
)

const (
	// MessageTypeHeader is the unique id of a tlv header.
	MessageTypeHeader message.Type = 0

	// HeaderTypeBytesLength defines the amount of bytes dedicated for the message type in the packet header.
	HeaderTypeBytesLength = 1

	// HeaderLengthByteLength defines the amount of bytes dedicated for the message length denotation in the packet header.
	HeaderLengthByteLength = 2

	// HeaderBytesLength defines the amount of bytes making up the protocol TLV packet header.
	HeaderBytesLength = HeaderLengthByteLength + HeaderTypeBytesLength
)

var (
	// HeaderMessageDefinition is the message header sent in each message denoting the TLV fields.
	HeaderMessageDefinition = &message.Definition{
		ID:             MessageTypeHeader,
		MaxBytesLength: HeaderBytesLength,
		VariableLength: false,
	}
)

// Header includes the definition of the message and its bytes length.
type Header struct {
	// The definition of the message.
	Definition *message.Definition
	// The length in bytes of the message.
	MessageBytesLength uint16
}

// WriteHeader writes a TLV header into the given Writer.
func WriteHeader(buf io.Writer, msgType message.Type, msgBytesLength uint16) error {
	if err := binary.Write(buf, binary.LittleEndian, byte(msgType)); err != nil {
		return err
	}

	return binary.Write(buf, binary.LittleEndian, msgBytesLength)
}

// ParseHeader parses the given buffer to a Header.
func ParseHeader(buf []byte, r *message.Registry) (*Header, error) {

	// get message
	def, err := r.DefinitionForType(message.Type(buf[0]))
	if err != nil {
		return nil, err
	}

	// extract length of message
	advMsgBytesLength := binary.LittleEndian.Uint16(buf[1:3])
	if (advMsgBytesLength > def.MaxBytesLength) || (!def.VariableLength && (advMsgBytesLength < def.MaxBytesLength)) {
		return nil, fmt.Errorf("%w: advertised length: %d bytes; max length: %d bytes", ErrInvalidMessageLength, advMsgBytesLength, def.MaxBytesLength)
	}

	return &Header{Definition: def, MessageBytesLength: advMsgBytesLength}, nil
}
