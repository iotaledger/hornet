package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	restapiHTTPErrorCount prometheus.Gauge
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

	registry.MustRegister(restapiHTTPErrorCount)

	addCollect(collectRestAPI)
}

func collectRestAPI() {
	restapiHTTPErrorCount.Set(float64(deps.RestAPIMetrics.HTTPRequestErrorCounter.Load()))
}
