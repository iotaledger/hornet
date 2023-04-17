package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	gossipBlocks         *prometheus.GaugeVec
	gossipRequests       *prometheus.GaugeVec
	gossipHeartbeats     *prometheus.GaugeVec
	gossipDroppedPackets *prometheus.GaugeVec
)

func configureGossipNode() {
	gossipBlocks = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "blocks",
			Help:      "Number of blocks.",
		},
		[]string{"type"},
	)

	gossipRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "requests",
			Help:      "Number of requests.",
		},
		[]string{"type"},
	)

	gossipHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "heartbeats",
			Help:      "Number of heartbeats.",
		},
		[]string{"type"},
	)

	gossipDroppedPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_node",
			Name:      "dropped_packets",
			Help:      "Number of dropped packets.",
		},
		[]string{"type"},
	)

	registry.MustRegister(gossipBlocks)
	registry.MustRegister(gossipRequests)
	registry.MustRegister(gossipHeartbeats)
	registry.MustRegister(gossipDroppedPackets)

	addCollect(collectServer)
}

func collectServer() {
	gossipBlocks.WithLabelValues("all").Set(float64(deps.ServerMetrics.Blocks.Load()))
	gossipBlocks.WithLabelValues("new").Set(float64(deps.ServerMetrics.NewBlocks.Load()))
	gossipBlocks.WithLabelValues("known").Set(float64(deps.ServerMetrics.KnownBlocks.Load()))
	gossipBlocks.WithLabelValues("referenced").Set(float64(deps.ServerMetrics.ReferencedBlocks.Load()))
	gossipBlocks.WithLabelValues("invalid").Set(float64(deps.ServerMetrics.InvalidBlocks.Load()))
	gossipBlocks.WithLabelValues("sent").Set(float64(deps.ServerMetrics.SentBlocks.Load()))
	gossipBlocks.WithLabelValues("sent_spam").Set(float64(deps.ServerMetrics.SentSpamBlocks.Load()))

	gossipRequests.WithLabelValues("invalid").Set(float64(deps.ServerMetrics.InvalidRequests.Load()))
	gossipRequests.WithLabelValues("received_blocks").Set(float64(deps.ServerMetrics.ReceivedBlockRequests.Load()))
	gossipRequests.WithLabelValues("received_milestones").Set(float64(deps.ServerMetrics.ReceivedMilestoneRequests.Load()))
	gossipRequests.WithLabelValues("sent_blocks").Set(float64(deps.ServerMetrics.SentBlockRequests.Load()))
	gossipRequests.WithLabelValues("sent_milestones").Set(float64(deps.ServerMetrics.SentMilestoneRequests.Load()))

	gossipHeartbeats.WithLabelValues("received").Set(float64(deps.ServerMetrics.ReceivedHeartbeats.Load()))
	gossipHeartbeats.WithLabelValues("sent").Set(float64(deps.ServerMetrics.SentHeartbeats.Load()))

	gossipDroppedPackets.WithLabelValues("sent").Set(float64(deps.ServerMetrics.DroppedPackets.Load()))
}
