package tangle

import (
	"github.com/iotaledger/hive.go/events"
)

type packageEvents struct {
	ReceivedValidMilestone *events.Event
	AddressSpent           *events.Event
}
