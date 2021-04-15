package metrics

import (
	"go.uber.org/atomic"
)

// DatabaseMetrics defines database metrics over the entire runtime of the node.
type DatabaseMetrics struct {
	// The total number of compactions.
	CompactionCount atomic.Uint32
	// Whether compaction is running or not.
	CompactionRunning atomic.Bool
}
