package pruning

import (
	"github.com/iotaledger/hive.go/events"
)

// PruningMetricsCaller is used to signal updated pruning metrics.
func PruningMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(metrics *PruningMetrics))(params[0].(*PruningMetrics))
}

type Events struct {
	PruningMilestoneIndexChanged *events.Event
	PruningMetricsUpdated        *events.Event
}
