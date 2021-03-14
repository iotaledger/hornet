package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	serverMessages           *prometheus.GaugeVec
	serverRequests           *prometheus.GaugeVec
	serverHeartbeats         *prometheus.GaugeVec
	serverDroppedSentPackets prometheus.Gauge
)

func configureServer() {
	serverMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_server_messages",
			Help: "Number of messages.",
		},
		[]string{"type"},
	)
	serverRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_server_requests",
			Help: "Number of requests.",
		},
		[]string{"type"},
	)
	serverHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_server_heartbeats",
			Help: "Number of heartbeats.",
		},
		[]string{"type"},
	)
	serverDroppedSentPackets = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_dropped_sent_packets",
		Help: "Number of dropped sent packets.",
	})

	registry.MustRegister(serverMessages)
	registry.MustRegister(serverRequests)
	registry.MustRegister(serverHeartbeats)
	registry.MustRegister(serverDroppedSentPackets)

	addCollect(collectServer)
}

func collectServer() {
	serverMessages.WithLabelValues("all").Set(float64(deps.ServerMetrics.Messages.Load()))
	serverMessages.WithLabelValues("new").Set(float64(deps.ServerMetrics.NewMessages.Load()))
	serverMessages.WithLabelValues("known").Set(float64(deps.ServerMetrics.KnownMessages.Load()))
	serverMessages.WithLabelValues("referenced").Set(float64(deps.ServerMetrics.ReferencedMessages.Load()))
	serverMessages.WithLabelValues("invalid").Set(float64(deps.ServerMetrics.InvalidMessages.Load()))
	serverMessages.WithLabelValues("sent").Set(float64(deps.ServerMetrics.SentMessages.Load()))
	serverMessages.WithLabelValues("sent_spam").Set(float64(deps.ServerMetrics.SentSpamMessages.Load()))
	serverMessages.WithLabelValues("validated").Set(float64(deps.ServerMetrics.ValidatedMessages.Load()))
	serverRequests.WithLabelValues("invalid").Set(float64(deps.ServerMetrics.InvalidRequests.Load()))
	serverRequests.WithLabelValues("received_message").Set(float64(deps.ServerMetrics.ReceivedMessageRequests.Load()))
	serverRequests.WithLabelValues("received_milestone").Set(float64(deps.ServerMetrics.ReceivedMilestoneRequests.Load()))
	serverRequests.WithLabelValues("sent_message").Set(float64(deps.ServerMetrics.SentMessageRequests.Load()))
	serverRequests.WithLabelValues("sent_milestone").Set(float64(deps.ServerMetrics.SentMilestoneRequests.Load()))
	serverHeartbeats.WithLabelValues("received").Set(float64(deps.ServerMetrics.ReceivedHeartbeats.Load()))
	serverHeartbeats.WithLabelValues("sent").Set(float64(deps.ServerMetrics.SentHeartbeats.Load()))
	serverDroppedSentPackets.Set(float64(deps.ServerMetrics.DroppedMessages.Load()))
}
