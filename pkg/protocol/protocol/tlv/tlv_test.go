package tlv_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol/message"
	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol/tlv"
)

const (
	MessageTypeTest message.Type = 1

	// length of a test message in bytes.
	TestMaxBytesLength = 5
)

var (
	TestMessageDefinition = &message.Definition{
		ID:             MessageTypeTest,
		MaxBytesLength: TestMaxBytesLength,
		VariableLength: false,
	}
	r = message.NewRegistry([]*message.Definition{
		tlv.HeaderMessageDefinition,
		TestMessageDefinition,
	})
)

func TestTLV(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength))

	err := tlv.WriteHeader(buf, MessageTypeTest, uint16(TestMaxBytesLength))
	assert.NoError(t, err)

	data := buf.Bytes()
	var header *tlv.Header
	header, err = tlv.ParseHeader(data, r)
	assert.NoError(t, err)

	assert.Equal(t, TestMessageDefinition, header.Definition)
	assert.Equal(t, uint16(TestMaxBytesLength), header.MessageBytesLength)
}

func TestTLV_ParseHeader(t *testing.T) {
	// unknown message type
	data := []byte{2, 5, 0}
	_, err := tlv.ParseHeader(data, r)
	assert.Error(t, err)

	// invalid message length
	data = []byte{byte(MessageTypeTest), 6, 0}
	_, err = tlv.ParseHeader(data, r)
	assert.Error(t, err)

}
