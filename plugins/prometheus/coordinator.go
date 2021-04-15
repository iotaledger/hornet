package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gohornet/hornet/pkg/model/coordinator"

	"github.com/iotaledger/hive.go/events"
)

var (
	coordinatorQuorumResponseTime       prometheus.Histogram
	coordinatorQuorumErrorCounter       prometheus.Counter
	coordinatorQuorumNodesResponseTimes *prometheus.HistogramVec
	coordinatorQuorumNodesErrorCounters *prometheus.CounterVec
	coordinatorSoftErrEncountered       prometheus.Counter
)

func configureCoordinator() {

	coordinatorQuorumResponseTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "coordinator",
			Name:      "quorum_duration",
			Help:      "Durations for complete quorum. [s]",
			Buckets:   prometheus.DefBuckets,
		})

	coordinatorQuorumErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "coordinator",
			Name:      "quorum_error_counter",
			Help:      "Number encountered errors for complete quorum.",
		})

	coordinatorQuorumNodesResponseTimes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "iota",
			Subsystem: "coordinator",
			Name:      "quorum_nodes_response_times",
			Help:      "Latest response time by quorum client. [s]",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"group", "alias", "baseURL"},
	)

	coordinatorQuorumNodesErrorCounters = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "coordinator",
			Name:      "quorum_nodes_error_counters",
			Help:      "Number encountered errors by quorum client.",
		},
		[]string{"group", "alias", "baseURL"},
	)

	coordinatorSoftErrEncountered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "coordinator",
			Name:      "soft_error_count",
			Help:      "The coordinator's encountered soft error count.",
		},
	)

	registry.MustRegister(coordinatorQuorumResponseTime)
	registry.MustRegister(coordinatorQuorumErrorCounter)
	registry.MustRegister(coordinatorQuorumNodesResponseTimes)
	registry.MustRegister(coordinatorQuorumNodesErrorCounters)
	registry.MustRegister(coordinatorSoftErrEncountered)

	deps.Coordinator.Events.QuorumFinished.Attach(events.NewClosure(func(result *coordinator.QuorumFinishedResult) {

		coordinatorQuorumResponseTime.Observe(result.Duration.Seconds())
		if result.Err != nil {
			coordinatorQuorumErrorCounter.Inc()
		}

		// get snapshot of last quorum stats
		for _, entry := range deps.Coordinator.QuorumStats() {
			labelsResponseTime := prometheus.Labels{
				"group":   entry.Group,
				"alias":   entry.Alias,
				"baseURL": entry.BaseURL,
			}
			coordinatorQuorumNodesResponseTimes.With(labelsResponseTime).Observe(entry.ResponseTimeSeconds)

			labelsErrorCounters := prometheus.Labels{
				"group":   entry.Group,
				"alias":   entry.Alias,
				"baseURL": entry.BaseURL,
			}

			if entry.Error != nil {
				coordinatorQuorumNodesErrorCounters.With(labelsErrorCounters).Inc()
			}
		}
	}))

	deps.Coordinator.Events.SoftError.Attach(events.NewClosure(func(_ error) {
		coordinatorSoftErrEncountered.Inc()
	}))
}
