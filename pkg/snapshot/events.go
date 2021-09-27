package snapshot

import (
	"github.com/iotaledger/hive.go/events"
)

// SnapshotMetricsCaller is used to signal updated snapshot metrics.
func SnapshotMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(metrics *SnapshotMetrics))(params[0].(*SnapshotMetrics))
}

// PruningMetricsCaller is used to signal updated pruning metrics.
func PruningMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(metrics *PruningMetrics))(params[0].(*PruningMetrics))
}

type Events struct {
	SnapshotMilestoneIndexChanged *events.Event
	SnapshotMetricsUpdated        *events.Event
	PruningMilestoneIndexChanged  *events.Event
	PruningMetricsUpdated         *events.Event
}
