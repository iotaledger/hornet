package pruning

import (
	"github.com/iotaledger/hive.go/runtime/event"
)


type Events struct {
	PruningMilestoneIndexChanged *event.Event1[uint32]
	PruningMetricsUpdated        *event.Event1[*Metrics]
}
