package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/core/events"
)

var (
	inxPoWCompletedCount prometheus.Gauge
	inxPoWBlockSizes     prometheus.Histogram
	inxPoWDurations      prometheus.Histogram
)

func configureINX() {

	inxPoWCompletedCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "inx",
			Name:      "pow_requests",
			Help:      "The amount of completed INX PoW requests.",
		},
	)

	inxPoWBlockSizes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "inx",
			Name:      "pow_block_sizes",
			Help:      "The block size of INX PoW requests.",
			Buckets:   powBlockSizeBuckets,
		})

	inxPoWDurations = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "inx",
			Name:      "pow_durations",
			Help:      "The duration of INX PoW requests [s].",
			Buckets:   powDurationBuckets,
		})

	registry.MustRegister(inxPoWCompletedCount)
	registry.MustRegister(inxPoWBlockSizes)
	registry.MustRegister(inxPoWDurations)

	deps.INXMetrics.Events.PoWCompleted.Hook(events.NewClosure(func(blockSize int, duration time.Duration) {
		inxPoWBlockSizes.Observe(float64(blockSize))
		inxPoWDurations.Observe(duration.Seconds())
	}))

	addCollect(collectINX)
}

func collectINX() {
	inxPoWCompletedCount.Set(float64(deps.INXMetrics.PoWCompletedCounter.Load()))
}
