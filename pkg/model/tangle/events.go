package tangle

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = packageEvents{
	ReceivedValidMilestone: events.NewEvent(MilestoneCaller),
	AddressSpent:           events.NewEvent(events.StringCaller),
}

type packageEvents struct {
	ReceivedValidMilestone *events.Event
	AddressSpent           *events.Event
}
