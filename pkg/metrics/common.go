package metrics

import (
	"time"
)

type PoWMetrics interface {
	PoWCompleted(blockSize int, duration time.Duration)
}
