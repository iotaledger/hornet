package snapshot

import (
	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	SnapshotMilestoneIndexChanged *events.Event
	PruningMilestoneIndexChanged  *events.Event
}
