package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/events"
)

var (
	coordinatorQuorumResponseTimes *prometheus.GaugeVec
	coordinatorQuorumErrorCounters *prometheus.GaugeVec
	coordinatorSoftErrEncountered  prometheus.Counter
)

func configureCoordinator() {
	coordinatorQuorumResponseTimes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_coordinator_quorum_response_times",
			Help: "Latest response time by quorum client.",
		},
		[]string{"group", "alias", "baseURL"},
	)

	coordinatorQuorumErrorCounters = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_coordinator_quorum_error_counters",
			Help: "Number encountered errors by quorum client.",
		},
		[]string{"group", "alias", "baseURL", "error"},
	)

	coordinatorSoftErrEncountered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_coordinator_soft_errors",
			Help: "The coordinator's encountered soft error count.",
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
		coordinatorQuorumResponseTimes.With(labelsResponseTime).Set(float64(entry.ResponseTimeMs))

		labelsErrorCounters := prometheus.Labels{
			"group":   entry.Group,
			"alias":   entry.Alias,
			"baseURL": entry.BaseURL,
			"error":   entry.Error.Error(),
		}
		coordinatorQuorumErrorCounters.With(labelsErrorCounters).Set(float64(entry.ErrorCounter))
	}
}
