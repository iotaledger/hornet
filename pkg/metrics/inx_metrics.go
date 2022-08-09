package metrics

import (
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/core/events"
)

type INXEvents struct {
	// PoWCompleted is fired when a PoW request is completed.
	PoWCompleted *events.Event
}

// INXMetrics defines INX metrics over the entire runtime of the node.
type INXMetrics struct {
	// The total number of completed PoW requests.
	PoWCompletedCounter atomic.Uint32

	Events *INXEvents
}

func (m *INXMetrics) PoWCompleted(blockSize int, duration time.Duration) {
	m.PoWCompletedCounter.Inc()
	if m.Events != nil && m.Events.PoWCompleted != nil {
		m.Events.PoWCompleted.Trigger(blockSize, duration)
	}
}
