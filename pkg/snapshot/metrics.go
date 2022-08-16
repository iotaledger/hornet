package snapshot

import (
	"time"
)

// Metrics holds metrics about a snapshot creation run.
type Metrics struct {
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
