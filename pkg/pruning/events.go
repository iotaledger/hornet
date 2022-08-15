package pruning

import (
	"github.com/iotaledger/hive.go/core/events"
)

// PruningMetricsCaller is used to signal updated pruning metrics.
func PruningMetricsCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(metrics *PruningMetrics))(params[0].(*PruningMetrics))
}

type Events struct {
	PruningMilestoneIndexChanged *events.Event
	PruningMetricsUpdated        *events.Event
}
