package prometheus

import (
	"time"

	echoprometheus "github.com/labstack/echo-contrib/prometheus"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/core/events"
)

var (
	restapiHTTPErrorCount prometheus.Gauge

	restapiPoWCompletedCount prometheus.Gauge
	restapiPoWBlockSizes     prometheus.Histogram
	restapiPoWDurations      prometheus.Histogram
)

func configureRestAPI() {
	restapiHTTPErrorCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "http_request_errors",
			Help:      "The amount of encountered HTTP request errors.",
		},
	)

	restapiPoWCompletedCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "pow_requests",
			Help:      "The amount of completed REST API PoW requests.",
		},
	)

	restapiPoWBlockSizes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "pow_block_sizes",
			Help:      "The block size of REST API PoW requests.",
			Buckets:   powBlockSizeBuckets,
		})

	restapiPoWDurations = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "restapi",
			Name:      "pow_durations",
			Help:      "The duration of REST API PoW requests [s].",
			Buckets:   powDurationBuckets,
		})

	registry.MustRegister(restapiHTTPErrorCount)

	registry.MustRegister(restapiPoWCompletedCount)
	registry.MustRegister(restapiPoWBlockSizes)
	registry.MustRegister(restapiPoWDurations)

	deps.RestAPIMetrics.Events.PoWCompleted.Hook(events.NewClosure(func(blockSize int, duration time.Duration) {
		restapiPoWBlockSizes.Observe(float64(blockSize))
		restapiPoWDurations.Observe(duration.Seconds())
	}))

	addCollect(collectRestAPI)

	if deps.Echo != nil {
		p := echoprometheus.NewPrometheus("iota_restapi", nil)
		for _, m := range p.MetricsList {
			registry.MustRegister(m.MetricCollector)
		}
		deps.Echo.Use(p.HandlerFunc)
	}
}

func collectRestAPI() {
	restapiHTTPErrorCount.Set(float64(deps.RestAPIMetrics.HTTPRequestErrorCounter.Load()))
	restapiPoWCompletedCount.Set(float64(deps.RestAPIMetrics.PoWCompletedCounter.Load()))
}
