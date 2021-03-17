package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/events"
)

var (
	coordinatorQuorumResponseTimes *prometheus.HistogramVec
	coordinatorQuorumErrorCounters *prometheus.GaugeVec
	coordinatorSoftErrEncountered  prometheus.Counter
)

func configureCoordinator() {

	coordinatorQuorumResponseTimes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "coordinator_quorum",
			Name:      "response_times",
			Help:      "Latest response time by quorum client.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"group", "alias", "baseURL"},
	)

	coordinatorQuorumErrorCounters = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "coordinator_quorum",
			Name:      "error_counters",
			Help:      "Number encountered errors by quorum client.",
		},
		[]string{"group", "alias", "baseURL"},
	)

	coordinatorSoftErrEncountered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "coordinator_quorum",
			Name:      "soft_error_count",
			Help:      "The coordinator's encountered soft error count.",
		},
	)

	registry.MustRegister(coordinatorQuorumResponseTimes)
	registry.MustRegister(coordinatorQuorumErrorCounters)
	registry.MustRegister(coordinatorSoftErrEncountered)

	deps.Coordinator.Events.SoftError.Attach(events.NewClosure(func(_ error) {
		coordinatorSoftErrEncountered.Inc()
	}))

	addCollect(collectCoordinatorQuorumStats)
}

func collectCoordinatorQuorumStats() {
	coordinatorQuorumResponseTimes.Reset()
	coordinatorQuorumErrorCounters.Reset()

	for _, entry := range deps.Coordinator.QuorumStats() {
		labelsResponseTime := prometheus.Labels{
			"group":   entry.Group,
			"alias":   entry.Alias,
			"baseURL": entry.BaseURL,
		}
		coordinatorQuorumResponseTimes.With(labelsResponseTime).Observe(entry.ResponseTimeSeconds)

		labelsErrorCounters := prometheus.Labels{
			"group":   entry.Group,
			"alias":   entry.Alias,
			"baseURL": entry.BaseURL,
		}
		coordinatorQuorumErrorCounters.With(labelsErrorCounters).Set(float64(entry.ErrorCounter))
	}
}
