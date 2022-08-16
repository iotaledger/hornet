package snapshot

import (
	"github.com/iotaledger/hive.go/core/events"
)

// MetricsCaller is used to signal updated snapshot metrics.
func MetricsCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(metrics *Metrics))(params[0].(*Metrics))
}

type Events struct {
	SnapshotMilestoneIndexChanged         *events.Event
	SnapshotMetricsUpdated                *events.Event
	HandledConfirmedMilestoneIndexChanged *events.Event
}
