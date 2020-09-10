package protocol

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync/atomic"

	"github.com/willf/bitset"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/protocol/handshake"
	"github.com/gohornet/hornet/pkg/protocol/message"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/pkg/protocol/tlv"
)

var (
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

	// supported protocol messages/feature sets
	SupportedFeatureSets = bitset.From([]uint64{sting.FeatureSet})
)

var (
	ownByteEncodedCooAddress []byte
	ownMWM                   uint64
	ownSrvSocketPort         uint16
)

// Init initializes the protocol package with the given handshake information.
func Init(cooAddressBytes []byte, mwm int, gossipBindAddr string) error {
	ownByteEncodedCooAddress = cooAddressBytes
	ownMWM = uint64(mwm)
	_, portStr, err := net.SplitHostPort(gossipBindAddr)
	if err != nil {
		return fmt.Errorf("gossip bind address is invalid: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("gossip bind address is invalid: %w", err)
	}
	ownSrvSocketPort = uint16(port)
	return nil
}

// Events holds protocol related events.
type Events struct {
	// Fired when a handshake was fully completed.
	HandshakeCompleted *events.Event
	// Holds event instances to attach to for received messages.
	// Use a message's ID to get the corresponding event.
	Received []*events.Event
	// Holds event instances to attach to for sent messages.
	// Use a message's ID to get the corresponding event.
	Sent []*events.Event
	// Fired for generic protocol errors.
	// It is suggested to close the underlying ReadWriteCloser of the Protocol instance
	// if any error occurs.
	Error *events.Event
}

// Protocol encapsulates the logic of parsing and sending protocol messages.
type Protocol struct {
	// The protocol features this instance supports.
	// This variable is only usable after protocol handshake.
	FeatureSet byte
	// Holds events for sent and received messages, handshake completion and generic errors.
	Events Events
	// the underlying connection
	conn io.ReadWriteCloser
	// the handshake state, 2 == completed
	handshakeState int32
	// handshaked is set after the handshake is completed
	handshaked bool
	// the current receiving message
	receivingMessage *message.Definition
	// the buffer holding the receiving message data
	receiveBuffer []byte
	// the current offset within the receiving buffer
	receiveBufferOffset int
	// mutex to synchronize multiple sends
	sendMutex syncutils.Mutex
}

// New generates a new protocol instance which is ready to read a first message header.
func New(conn io.ReadWriteCloser) *Protocol {

	// load message definitions
	definitions := message.Definitions()

	// allocate event handlers for all message types
	receiveHandlers := make([]*events.Event, len(definitions))
	sentHandlers := make([]*events.Event, len(definitions))
	for i, def := range definitions {
		if def == nil {
			continue
		}
		receiveHandlers[i] = events.NewEvent(events.ByteSliceCaller)
		sentHandlers[i] = events.NewEvent(events.CallbackCaller)
	}

	protocol := &Protocol{
		conn: conn,
		Events: Events{
			HandshakeCompleted: events.NewEvent(events.CallbackCaller),
			Received:           receiveHandlers,
			Sent:               sentHandlers,
			Error:              events.NewEvent(events.ErrorCaller),
		},
		// the first message on the protocol is a TLV header
		receiveBuffer:    make([]byte, tlv.HeaderMessageDefinition.MaxBytesLength),
		receivingMessage: tlv.HeaderMessageDefinition,
	}

	return protocol
}

// Supports tells whether the protocol supports the given feature set.
func (p *Protocol) Supports(featureSet byte) bool {
	return p.FeatureSet&featureSet > 0
}

// SupportedFeatureSets returns a slice of named supported feature sets.
func (p *Protocol) SupportedFeatureSets() []string {
	var features []string
	if p.Supports(sting.FeatureSet) {
		features = append(features, sting.FeatureSetName)
	}
	return features
}

// Start kicks off the protocol by sending a handshake message and starting to read from
// the connection.
func (p *Protocol) Start() {
	// kick off protocol by sending a handshake message
	handshakeMsg, err := handshake.NewHandshakeMessage(SupportedFeatureSets, ownSrvSocketPort, ownByteEncodedCooAddress, byte(ownMWM))
	if err != nil {
		fmt.Println("creating handshake message error: ", err)
		_ = p.conn.Close()
		p.Events.Error.Trigger(err)
		return
	}

	if err := p.Send(handshakeMsg); err != nil {
		fmt.Println("sending handshake message error: ", err)
		_ = p.conn.Close()
		p.Events.Error.Trigger(err)
		return
	}

	// start reading from the connection
	_, _ = p.conn.Read(make([]byte, 2048))
}

// Handshaked has to be called when a handshake message was received (and finalized) and sent.
// If this function is called twice, it will trigger a HandshakeCompleted event.
func (p *Protocol) Handshaked() {
	if atomic.AddInt32(&p.handshakeState, 1) == 2 {
		p.Events.HandshakeCompleted.Trigger()
		p.handshaked = true
	}
}

// IsHandshaked tells if the peer is handshaked.
// it is set after the HandshakeCompleted event has been fired.
func (p *Protocol) IsHandshaked() bool {
	return p.handshaked
}

// Receive acts as an event handler for received data.
func (p *Protocol) Receive(data []byte) {
	offset := 0
	length := len(data)

	// continue to parse messages as long as we have data to consume
	for offset < length && p.receivingMessage != nil {

		// read in data into the receive buffer for the current message type
		bytesRead := byteutils.ReadAvailableBytesToBuffer(p.receiveBuffer, p.receiveBufferOffset, data, offset, length)

		p.receiveBufferOffset += bytesRead

		// advance consumed offset of received data
		offset += bytesRead

		if p.receiveBufferOffset != len(p.receiveBuffer) {
			return
		}

		// message fully received
		p.receiveBufferOffset = 0

		// interpret the next message type if we received a header
		if p.receivingMessage.ID == tlv.HeaderMessageDefinition.ID {

			header, err := tlv.ParseHeader(p.receiveBuffer)
			if err != nil {
				p.Events.Error.Trigger(err)
				_ = p.conn.Close()
				return
			}

			// advance to handle the message type the header says we are receiving
			p.receivingMessage = header.Definition

			// allocate enough space for it
			p.receiveBuffer = make([]byte, header.MessageBytesLength)
			continue
		}

		// fire the message type's event handler.
		// note that the message id is valid here because we verified that the message type
		// exists while parsing the TLV header
		p.Events.Received[p.receivingMessage.ID].Trigger(p.receiveBuffer)

		// reset to receiving a header
		p.receivingMessage = tlv.HeaderMessageDefinition
		p.receiveBuffer = make([]byte, tlv.HeaderMessageDefinition.MaxBytesLength)
	}
}

// Send sends the given message (including the message header) to the underlying writer.
// It fires the corresponding send event for the specific message type.
func (p *Protocol) Send(message []byte) error {
	p.sendMutex.Lock()
	defer p.sendMutex.Unlock()

	// write message
	if _, err := p.conn.Write(message); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// fire event handler for sent message
	p.Events.Sent[message[0]].Trigger()

	return nil
}
