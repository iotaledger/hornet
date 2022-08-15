package snapshot

import (
	"github.com/iotaledger/hive.go/core/events"
)

// SnapshotMetricsCaller is used to signal updated snapshot metrics.
func SnapshotMetricsCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(metrics *SnapshotMetrics))(params[0].(*SnapshotMetrics))
}

type Events struct {
	SnapshotMilestoneIndexChanged         *events.Event
	SnapshotMetricsUpdated                *events.Event
	HandledConfirmedMilestoneIndexChanged *events.Event
}
