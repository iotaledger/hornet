package snapshot

import (
	"github.com/iotaledger/hive.go/runtime/event"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Events struct {
	SnapshotMilestoneIndexChanged         *event.Event1[iotago.MilestoneIndex]
	SnapshotMetricsUpdated                *event.Event1[*Metrics]
	HandledConfirmedMilestoneIndexChanged *event.Event1[iotago.MilestoneIndex]
}

func newEvents() *Events {
	return &Events{
		SnapshotMilestoneIndexChanged:         event.New1[iotago.MilestoneIndex](),
		SnapshotMetricsUpdated:                event.New1[*Metrics](),
		HandledConfirmedMilestoneIndexChanged: event.New1[iotago.MilestoneIndex](),
	}
}
