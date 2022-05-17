package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	gossipMessages       *prometheus.GaugeVec
	gossipRequests       *prometheus.GaugeVec
	gossipHeartbeats     *prometheus.GaugeVec
	gossipDroppedPackets *prometheus.GaugeVec
)

func configureGossipNode() {
	gossipMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "message_count",
			Help:      "Number of messages.",
		},
		[]string{"type"},
	)

	gossipRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "request_count",
			Help:      "Number of requests.",
		},
		[]string{"type"},
	)

	gossipHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "heartbeat_count",
			Help:      "Number of heartbeats.",
		},
		[]string{"type"},
	)

	gossipDroppedPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "dropped_count",
			Help:      "Number of dropped packets.",
		},
		[]string{"type"},
	)

	registry.MustRegister(gossipMessages)
	registry.MustRegister(gossipRequests)
	registry.MustRegister(gossipHeartbeats)
	registry.MustRegister(gossipDroppedPackets)

	addCollect(collectServer)
}

func collectServer() {
	gossipMessages.WithLabelValues("all").Set(float64(deps.ServerMetrics.Blocks.Load()))
	gossipMessages.WithLabelValues("new").Set(float64(deps.ServerMetrics.NewBlocks.Load()))
	gossipMessages.WithLabelValues("known").Set(float64(deps.ServerMetrics.KnownBlocks.Load()))
	gossipMessages.WithLabelValues("referenced").Set(float64(deps.ServerMetrics.ReferencedBlocks.Load()))
	gossipMessages.WithLabelValues("invalid").Set(float64(deps.ServerMetrics.InvalidBlocks.Load()))
	gossipMessages.WithLabelValues("sent").Set(float64(deps.ServerMetrics.SentBlocks.Load()))
	gossipMessages.WithLabelValues("sent_spam").Set(float64(deps.ServerMetrics.SentSpamBlocks.Load()))
	gossipMessages.WithLabelValues("validated").Set(float64(deps.ServerMetrics.ValidatedBlocks.Load()))

	gossipRequests.WithLabelValues("invalid").Set(float64(deps.ServerMetrics.InvalidRequests.Load()))
	gossipRequests.WithLabelValues("received_message").Set(float64(deps.ServerMetrics.ReceivedBlockRequests.Load()))
	gossipRequests.WithLabelValues("received_milestone").Set(float64(deps.ServerMetrics.ReceivedMilestoneRequests.Load()))
	gossipRequests.WithLabelValues("sent_message").Set(float64(deps.ServerMetrics.SentBlockRequests.Load()))
	gossipRequests.WithLabelValues("sent_milestone").Set(float64(deps.ServerMetrics.SentMilestoneRequests.Load()))

	gossipHeartbeats.WithLabelValues("received").Set(float64(deps.ServerMetrics.ReceivedHeartbeats.Load()))
	gossipHeartbeats.WithLabelValues("sent").Set(float64(deps.ServerMetrics.SentHeartbeats.Load()))

	gossipDroppedPackets.WithLabelValues("sent").Set(float64(deps.ServerMetrics.DroppedPackets.Load()))
}
