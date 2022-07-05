package snapshot

import (
	"github.com/iotaledger/hive.go/events"
)

// SnapshotMetricsCaller is used to signal updated snapshot metrics.
func SnapshotMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(metrics *SnapshotMetrics))(params[0].(*SnapshotMetrics))
}

type Events struct {
	SnapshotMilestoneIndexChanged         *events.Event
	SnapshotMetricsUpdated                *events.Event
	HandledConfirmedMilestoneIndexChanged *events.Event
}
