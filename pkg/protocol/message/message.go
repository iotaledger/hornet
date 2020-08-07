package message

import (
	"errors"
)

var (
	// ErrTypeAlreadyDefined is returned when an already defined message type is redefined.
	ErrTypeAlreadyDefined = errors.New("message type is already defined")
	// ErrUnknownType is returned when a definition for an unknown message type is is requested.
	ErrUnknownType = errors.New("message type unknown")
)

// Type denotes the byte ID of a given message type.
type Type byte

// Definition describes a message's ID and its max byte length (and whether the length variable).
type Definition struct {
	ID             Type
	MaxBytesLength uint16
	// VariableLength defines if the message is fixed size or not
	// if it is fixed, the message length must be MaxBytesLength
	VariableLength bool
}

var definitions = make([]*Definition, 0)

// Definitions returns all registered message definitions.
func Definitions() []*Definition {
	return definitions
}

// RegisterType registers the given message type with its definition.
func RegisterType(msgType Type, def *Definition) error {
	// grow definitions slice appropriately
	if len(definitions)-1 < int(msgType) {
		definitionsCopy := make([]*Definition, int(msgType)+1)
		copy(definitionsCopy, definitions)
		definitions = definitionsCopy
	}
	if definitions[msgType] != nil {
		return ErrTypeAlreadyDefined
	}
	definitions[msgType] = def
	return nil
}

// DefinitionForType returns the definition for the given message type.
func DefinitionForType(msgType Type) (*Definition, error) {
	if len(definitions)-1 < int(msgType) {
		return nil, ErrUnknownType
	}
	def := definitions[msgType]
	if def == nil {
		return nil, ErrUnknownType
	}
	return def, nil
}
