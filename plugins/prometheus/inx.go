package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/events"
)

var (
	inxPoWCompletedCount prometheus.Gauge
	inxPoWMessageSizes   prometheus.Histogram
	inxPoWDurations      prometheus.Histogram
)

func configureINX() {

	inxPoWCompletedCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "inx",
			Name:      "pow_count",
			Help:      "The amount of completed INX PoW requests.",
		},
	)

	inxPoWMessageSizes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "inx",
			Name:      "pow_message_sizes",
			Help:      "The message size of INX PoW requests.",
			Buckets:   powMessageSizeBuckets,
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
	registry.MustRegister(inxPoWMessageSizes)
	registry.MustRegister(inxPoWDurations)

	deps.INXMetrics.Events.PoWCompleted.Attach(events.NewClosure(func(messageSize int, duration time.Duration) {
		inxPoWMessageSizes.Observe(float64(messageSize))
		inxPoWDurations.Observe(duration.Seconds())
	}))

	addCollect(collectINX)
}

func collectINX() {
	inxPoWCompletedCount.Set(float64(deps.INXMetrics.PoWCompletedCounter.Load()))
}
