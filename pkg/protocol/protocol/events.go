package protocol

import (
	"github.com/iotaledger/hive.go/runtime/event"
)

// Events holds protocol related events.
type Events struct {
	// Holds event instances to attach to for received messages.
	// Use a message's ID to get the corresponding event.
	Received []*event.Event1[[]byte]
	// Fired for generic protocol errors.
	Error *event.Event1[error]
}
