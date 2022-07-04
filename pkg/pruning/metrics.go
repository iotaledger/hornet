package pruning

import (
	"time"
)

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
