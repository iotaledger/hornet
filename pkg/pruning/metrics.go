package pruning

import (
	"time"
)

// Metrics holds metrics about a database pruning run.
type Metrics struct {
	DurationPruneUnreferencedBlocks      time.Duration
	DurationTraverseMilestoneCone        time.Duration
	DurationPruneMilestone               time.Duration
	DurationPruneBlocks                  time.Duration
	DurationSetSnapshotInfo              time.Duration
	DurationPruningMilestoneIndexChanged time.Duration
	DurationTotal                        time.Duration
}
