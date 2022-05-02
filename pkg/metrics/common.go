package metrics

import (
	"time"
)

type PoWMetrics interface {
	PoWCompleted(messageSize int, duration time.Duration)
}
