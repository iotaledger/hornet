package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	gossipPeersMessages       *prometheus.GaugeVec
	gossipPeersRequests       *prometheus.GaugeVec
	gossipPeersHeartbeats     *prometheus.GaugeVec
	gossipPeersDroppedPackets *prometheus.GaugeVec
	gossipPeersConnected      *prometheus.GaugeVec
)

func configureGossipPeers() {

	gossipPeersMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "message_count",
			Help:      "Number of messages by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "request_count",
			Help:      "Number of requests by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "heartbeat_count",
			Help:      "Number of heartbeats by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersDroppedPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "dropped_count",
			Help:      "Number of dropped packets by peer.",
		},
		[]string{"address", "alias", "id", "type"},
	)

	gossipPeersConnected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "gossip_peers",
			Name:      "connected_count",
			Help:      "Are peers connected?",
		},
		[]string{"address", "alias", "id"},
	)

	registry.MustRegister(gossipPeersMessages)
	registry.MustRegister(gossipPeersRequests)
	registry.MustRegister(gossipPeersHeartbeats)
	registry.MustRegister(gossipPeersDroppedPackets)
	registry.MustRegister(gossipPeersConnected)

	addCollect(collectGossipPeers)
}

func collectGossipPeers() {
	gossipPeersMessages.Reset()
	gossipPeersRequests.Reset()
	gossipPeersHeartbeats.Reset()
	gossipPeersDroppedPackets.Reset()
	gossipPeersConnected.Reset()

	for _, peer := range deps.Manager.PeerInfoSnapshots() {

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

		gossipProto := deps.Service.Protocol(peer.Peer.ID)
		if gossipProto == nil {
			continue
		}

		gossipMessages.With(getLabels("all")).Set(float64(gossipProto.Metrics.ReceivedMessages.Load()))
		gossipMessages.With(getLabels("new")).Set(float64(gossipProto.Metrics.NewMessages.Load()))
		gossipMessages.With(getLabels("known")).Set(float64(gossipProto.Metrics.KnownMessages.Load()))
		gossipMessages.With(getLabels("sent")).Set(float64(gossipProto.Metrics.SentMessages.Load()))

		gossipRequests.With(getLabels("received_messages")).Set(float64(gossipProto.Metrics.ReceivedMessageRequests.Load()))
		gossipRequests.With(getLabels("received_milestones")).Set(float64(gossipProto.Metrics.ReceivedMilestoneRequests.Load()))
		gossipRequests.With(getLabels("sent_messages")).Set(float64(gossipProto.Metrics.SentMessageRequests.Load()))
		gossipRequests.With(getLabels("sent_milestones")).Set(float64(gossipProto.Metrics.SentMilestoneRequests.Load()))

		gossipHeartbeats.With(getLabels("received")).Set(float64(gossipProto.Metrics.ReceivedHeartbeats.Load()))
		gossipHeartbeats.With(getLabels("sent")).Set(float64(gossipProto.Metrics.SentHeartbeats.Load()))

		gossipDroppedPackets.With(getLabels("sent")).Set(float64(peer.DroppedSentPackets))

		gossipPeersConnected.With(peerLabels).Set(0)
		if peer.Connected {
			gossipPeersConnected.With(peerLabels).Set(1)
		}
	}
}
