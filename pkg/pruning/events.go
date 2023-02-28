package pruning

import (
	"github.com/iotaledger/hive.go/runtime/event"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Events struct {
	PruningMilestoneIndexChanged *event.Event1[iotago.MilestoneIndex]
	PruningMetricsUpdated        *event.Event1[*Metrics]
}

func newEvents() *Events {
	return &Events{
		PruningMilestoneIndexChanged: event.New1[iotago.MilestoneIndex](),
		PruningMetricsUpdated:        event.New1[*Metrics](),
	}
}
