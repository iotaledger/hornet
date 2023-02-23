package protocol_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol"
	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol/message"
	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol/tlv"
)

const (
	testMessageType message.Type = 1
)

var (
	testMessageDefinition = &message.Definition{
		ID:             testMessageType,
		MaxBytesLength: math.MaxUint16,
		VariableLength: true,
	}
	msgRegistry = message.NewRegistry([]*message.Definition{
		tlv.HeaderMessageDefinition,
		testMessageDefinition,
	})

	testMessage = []byte("test!")
)

func newTestPacket() ([]byte, error) {
	buf := new(bytes.Buffer)
	// write tlv header into buffer
	if err := tlv.WriteHeader(buf, testMessageType, uint16(len(testMessage))); err != nil {
		return nil, err
	}
	// write serialized packet bytes into the buffer
	if err := binary.Write(buf, binary.BigEndian, testMessage); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestProtocol_Read(t *testing.T) {
	p := protocol.New(msgRegistry)

	var receivedMessages [][]byte
	p.Events.Received[testMessageDefinition.ID].Hook(func(message []byte) {
		receivedMessages = append(receivedMessages, message)
	})

	pkt, err := newTestPacket()
	assert.NoError(t, err)

	n, err := p.Read(pkt)
	assert.Equal(t, len(pkt), n)
	assert.NoError(t, err)

	// check the event
	assert.ElementsMatch(t, [][]byte{testMessage}, receivedMessages)
}

func TestProtocol_ReadTwice(t *testing.T) {
	p := protocol.New(msgRegistry)

	var receivedMessages [][]byte
	p.Events.Received[testMessageDefinition.ID].Hook(func(message []byte) {
		receivedMessages = append(receivedMessages, message)
	})

	var buf bytes.Buffer

	pkt, err := newTestPacket()
	assert.NoError(t, err)
	buf.Write(pkt)
	pkt, err = newTestPacket()
	assert.NoError(t, err)
	buf.Write(pkt)

	n, err := p.Read(buf.Bytes())
	assert.Equal(t, buf.Len(), n)
	assert.NoError(t, err)

	// check the event
	assert.ElementsMatch(t, [][]byte{testMessage, testMessage}, receivedMessages)
}

func TestProtocol_ReadSplit(t *testing.T) {
	p := protocol.New(msgRegistry)

	var receivedMessages [][]byte
	p.Events.Received[testMessageDefinition.ID].Hook(func(message []byte) {
		receivedMessages = append(receivedMessages, message)
	})

	pkt, err := newTestPacket()
	assert.NoError(t, err)

	for _, b := range pkt {
		n, err := p.Read([]byte{b})
		assert.Equal(t, 1, n)
		assert.NoError(t, err)
	}

	// check the event
	assert.ElementsMatch(t, [][]byte{testMessage}, receivedMessages)
}
