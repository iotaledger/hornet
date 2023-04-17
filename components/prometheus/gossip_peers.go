package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	gossipPeersBlocks         *prometheus.GaugeVec
	gossipPeersRequests       *prometheus.GaugeVec
	gossipPeersHeartbeats     *prometheus.GaugeVec
	gossipPeersDroppedPackets *prometheus.GaugeVec
	gossipPeersConnected      *prometheus.GaugeVec
)

func configureGossipPeers() {

	gossipPeersBlocks = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "blocks",
			Help:      "Number of blocks by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "requests",
			Help:      "Number of requests by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "heartbeats",
			Help:      "Number of heartbeats by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersDroppedPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "dropped_packets",
			Help:      "Number of dropped packets by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersConnected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "connected_peers",
			Help:      "Are peers connected?",
		},
		[]string{"address", "alias", "id"},
	)

	registry.MustRegister(gossipPeersBlocks)
	registry.MustRegister(gossipPeersRequests)
	registry.MustRegister(gossipPeersHeartbeats)
	registry.MustRegister(gossipPeersDroppedPackets)
	registry.MustRegister(gossipPeersConnected)

	addCollect(collectGossipPeers)
}

func collectGossipPeers() {
	gossipPeersBlocks.Reset()
	gossipPeersRequests.Reset()
	gossipPeersHeartbeats.Reset()
	gossipPeersDroppedPackets.Reset()
	gossipPeersConnected.Reset()

	for _, peer := range deps.PeeringManager.PeerInfoSnapshots() {

		peerLabels := prometheus.Labels{
			"id":      peer.ID,
			"address": peer.Addresses[0].String(),
			"alias":   peer.Alias,
		}

		getLabels := func(messageType string) prometheus.Labels {
			return prometheus.Labels{
				"id":      peer.ID,
				"address": peer.Addresses[0].String(),
				"alias":   peer.Alias,
				"type":    messageType,
			}
		}

		gossipProto := deps.GossipService.Protocol(peer.Peer.ID)
		if gossipProto == nil {
			continue
		}

		gossipPeersBlocks.With(getLabels("all")).Set(float64(gossipProto.Metrics.ReceivedBlocks.Load()))
		gossipPeersBlocks.With(getLabels("new")).Set(float64(gossipProto.Metrics.NewBlocks.Load()))
		gossipPeersBlocks.With(getLabels("known")).Set(float64(gossipProto.Metrics.KnownBlocks.Load()))
		gossipPeersBlocks.With(getLabels("sent")).Set(float64(gossipProto.Metrics.SentBlocks.Load()))

		gossipPeersRequests.With(getLabels("received_blocks")).Set(float64(gossipProto.Metrics.ReceivedBlockRequests.Load()))
		gossipPeersRequests.With(getLabels("received_milestones")).Set(float64(gossipProto.Metrics.ReceivedMilestoneRequests.Load()))
		gossipPeersRequests.With(getLabels("sent_blocks")).Set(float64(gossipProto.Metrics.SentBlockRequests.Load()))
		gossipPeersRequests.With(getLabels("sent_milestones")).Set(float64(gossipProto.Metrics.SentMilestoneRequests.Load()))

		gossipPeersHeartbeats.With(getLabels("received")).Set(float64(gossipProto.Metrics.ReceivedHeartbeats.Load()))
		gossipPeersHeartbeats.With(getLabels("sent")).Set(float64(gossipProto.Metrics.SentHeartbeats.Load()))

		gossipPeersDroppedPackets.With(getLabels("sent")).Set(float64(peer.DroppedSentPackets))

		gossipPeersConnected.With(peerLabels).Set(0)
		if peer.Connected {
			gossipPeersConnected.With(peerLabels).Set(1)
		}
	}
}
