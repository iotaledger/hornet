package message_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hornet/v2/pkg/protocol/protocol/message"
)

// message definition for testing.
var (
	DummyMessageType       message.Type
	DummyMessageDefinition = &message.Definition{
		ID:             DummyMessageType,
		MaxBytesLength: 10,
		VariableLength: false,
	}
)

func TestMessage_NewRegistry(t *testing.T) {
	r := message.NewRegistry([]*message.Definition{DummyMessageDefinition})
	definitions := r.Definitions()
	assert.Equal(t, definitions[0], DummyMessageDefinition)

	// start with msg type 1 instead of 0
	DummyMessageDefinition1 := &message.Definition{
		ID:             1,
		MaxBytesLength: 10,
		VariableLength: false,
	}
	assert.Panics(t, func() {
		r = message.NewRegistry([]*message.Definition{DummyMessageDefinition1})
	})

	// skip one index
	DummyMessageDefinition3 := &message.Definition{
		ID:             3,
		MaxBytesLength: 10,
		VariableLength: false,
	}
	assert.Panics(t, func() {
		r = message.NewRegistry([]*message.Definition{
			DummyMessageDefinition,
			DummyMessageDefinition1,
			DummyMessageDefinition3,
		})
	})

	// try to init with empty
	assert.Panics(t, func() {
		r = message.NewRegistry([]*message.Definition{})
	})
}

func TestMessage_DefinitionForType(t *testing.T) {
	r := message.NewRegistry([]*message.Definition{DummyMessageDefinition})
	def, err := r.DefinitionForType(DummyMessageType)
	assert.NoError(t, err)
	assert.Equal(t, DummyMessageDefinition, def)
}
