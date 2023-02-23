package metrics

import (
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/runtime/event"
)

type RestAPIEvents struct {
	// PoWCompleted is fired when a PoW request is completed. It contains the block size and the duration.
	PoWCompleted *event.Event2[int, time.Duration]
}

// RestAPIMetrics defines REST API metrics over the entire runtime of the node.
type RestAPIMetrics struct {
	// The total number of HTTP request errors.
	HTTPRequestErrorCounter atomic.Uint32
	// The total number of completed PoW requests.
	PoWCompletedCounter atomic.Uint32

	Events *RestAPIEvents
}

func (m *RestAPIMetrics) PoWCompleted(blockSize int, duration time.Duration) {
	m.PoWCompletedCounter.Inc()
	if m.Events != nil && m.Events.PoWCompleted != nil {
		m.Events.PoWCompleted.Trigger(blockSize, duration)
	}
}
