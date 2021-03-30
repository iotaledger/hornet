package snapshot

import (
	"time"

	"github.com/iotaledger/hive.go/events"
)

// SnapshotMetrics holds metrics about a snapshot creation run.
type SnapshotMetrics struct {
	DurationReadLockLedger                time.Duration
	DurationInit                          time.Duration
	DurationSetSnapshotInfo               time.Duration
	DurationSnapshotMilestoneIndexChanged time.Duration
	DurationHeader                        time.Duration
	DurationSolidEntryPoints              time.Duration
	DurationOutputs                       time.Duration
	DurationMilestoneDiffs                time.Duration
	DurationTotal                         time.Duration
}

// SnapshotMetricsCaller is used to signal updated snapshot metrics.
func SnapshotMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(metrics *SnapshotMetrics))(params[0].(*SnapshotMetrics))
}

// PruningMetrics holds metrics about a database pruning run.
type PruningMetrics struct {
	DurationPruneUnreferencedMessages    time.Duration
	DurationTraverseMilestoneCone        time.Duration
	DurationPruneMilestone               time.Duration
	DurationPruneMessages                time.Duration
	DurationSetSnapshotInfo              time.Duration
	DurationPruningMilestoneIndexChanged time.Duration
	DurationTotal                        time.Duration
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
