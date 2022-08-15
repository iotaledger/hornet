package pruning

import (
	"github.com/iotaledger/hive.go/core/events"
)

// MetricsCaller is used to signal updated pruning metrics.
func MetricsCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(metrics *Metrics))(params[0].(*Metrics))
}

type Events struct {
	PruningMilestoneIndexChanged *events.Event
	PruningMetricsUpdated        *events.Event
}
