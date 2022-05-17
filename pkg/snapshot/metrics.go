package snapshot

import (
	"time"
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

// PruningMetrics holds metrics about a database pruning run.
type PruningMetrics struct {
	DurationPruneUnreferencedBlocks      time.Duration
	DurationTraverseMilestoneCone        time.Duration
	DurationPruneMilestone               time.Duration
	DurationPruneBlocks                  time.Duration
	DurationSetSnapshotInfo              time.Duration
	DurationPruningMilestoneIndexChanged time.Duration
	DurationTotal                        time.Duration
}
