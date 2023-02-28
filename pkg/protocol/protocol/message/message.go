package message

import (
	"errors"
)

var (
	// ErrTypeAlreadyDefined is returned when an already defined message type is redefined.
	ErrTypeAlreadyDefined = errors.New("message type is already defined")
	// ErrUnknownType is returned when a definition for an unknown message type is requested.
	ErrUnknownType = errors.New("message type unknown")
)

// Type denotes the byte ID of a given message type.
type Type byte

// Definition describes a message's ID, its max byte length and whether its size can be variable.
type Definition struct {
	// ID defines the unique identifier of the message.
	ID Type
	// MaxBytesLength defines the max byte length of the message type.
	// when 0, it means a message can be arbitrary size
	MaxBytesLength uint16
	// VariableLength defines if the message length is variable.
	VariableLength bool
}

// Registry holds message definitions.
type Registry struct {
	definitions []*Definition
}

// NewRegistry creates and initializes a new Registry.
// Once it is done, the Registry is immutable.
// Message definitions should be strictly monotonically increasing (based on their Message Type (uint16)).
func NewRegistry(defs []*Definition) *Registry {
	if len(defs) == 0 {
		panic("can't initialize registry with empty definitions")
	}
	// create a Registry with room for all definitions
	r := &Registry{definitions: make([]*Definition, len(defs))}

	for i, def := range defs {
		// check order and monotonicity of definitions
		if Type(i) != def.ID {
			panic("message definitions are inconsistent")
		}
		// add definition to Registry
		r.definitions[def.ID] = def
	}

	return r
}

// Definitions returns all registered message definitions.
func (r *Registry) Definitions() []*Definition {
	return r.definitions
}

// DefinitionForType returns the definition for the given message type.
func (r *Registry) DefinitionForType(msgType Type) (*Definition, error) {
	if len(r.definitions)-1 < int(msgType) {
		return nil, ErrUnknownType
	}
	def := r.definitions[msgType]
	if def == nil {
		return nil, ErrUnknownType
	}

	return def, nil
}
