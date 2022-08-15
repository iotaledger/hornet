package metrics

import (
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/core/events"
)

func PoWCompletedCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(blockSize int, duration time.Duration))(params[0].(int), params[1].(time.Duration))
}

type RestAPIEvents struct {
	// PoWCompleted is fired when a PoW request is completed.
	PoWCompleted *events.Event
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
