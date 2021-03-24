package metrics

import (
	"go.uber.org/atomic"
)

// StorageMetrics defines storage metrics over the entire runtime of the node.
type StorageMetrics struct {
	// The total number of prunings.
	Prunings atomic.Uint32
	// Whether pruning is running or not.
	PruningRunning atomic.Bool
}
