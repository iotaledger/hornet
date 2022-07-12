package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/events"
)

var (
	/*
		MWM: 10, Length: 8
		MWM: 11, Length: 15
		MWM: 12, Length: 45
		MWM: 13, Length: 133
		MWM: 14, Length: 399
		MWM: 15, Length: 1196
		MWM: 16, Length: 3588
		MWM: 17, Length: 10762
		MWM: 18, Length: 32286
	*/
	restapiPoWMessageSizeBuckets = []float64{
		45,
		133,
		399,
		1196,
		3588,
		10762,
		32286,
	}
	restapiPoWDurationBuckets = []float64{.1, .2, .5, 1, 2, 5, 10, 20, 50, 100, 200, 500}

	restapiHTTPErrorCount prometheus.Gauge

	restapiPoWCompletedCount prometheus.Gauge
	restapiPoWMessageSizes   prometheus.Histogram
	restapiPoWDurations      prometheus.Histogram
)

func configureRestAPI() {
	restapiHTTPErrorCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "http_request_error_count",
			Help:      "The amount of encountered HTTP request errors.",
		},
	)

	restapiPoWCompletedCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "pow_count",
			Help:      "The amount of completed REST API PoW requests.",
		},
	)

	restapiPoWMessageSizes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "pow_message_sizes",
			Help:      "The message size of REST API PoW requests.",
			Buckets:   restapiPoWMessageSizeBuckets,
		})

	restapiPoWDurations = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "pow_durations",
			Help:      "The duration of REST API PoW requests [s].",
			Buckets:   restapiPoWDurationBuckets,
		})

	registry.MustRegister(restapiHTTPErrorCount)

	registry.MustRegister(restapiPoWCompletedCount)
	registry.MustRegister(restapiPoWMessageSizes)
	registry.MustRegister(restapiPoWDurations)

	deps.RestAPIMetrics.Events.PoWCompleted.Hook(events.NewClosure(func(messageSize int, duration time.Duration) {
		restapiPoWMessageSizes.Observe(float64(messageSize))
		restapiPoWDurations.Observe(duration.Seconds())
	}))

	addCollect(collectRestAPI)
}

func collectRestAPI() {
	restapiHTTPErrorCount.Set(float64(deps.RestAPIMetrics.HTTPRequestErrorCounter.Load()))
	restapiPoWCompletedCount.Set(float64(deps.RestAPIMetrics.PoWCompletedCounter.Load()))
}
