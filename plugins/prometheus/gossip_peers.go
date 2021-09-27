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

		gossipPeersMessages.With(getLabels("all")).Set(float64(gossipProto.Metrics.ReceivedMessages.Load()))
		gossipPeersMessages.With(getLabels("new")).Set(float64(gossipProto.Metrics.NewMessages.Load()))
		gossipPeersMessages.With(getLabels("known")).Set(float64(gossipProto.Metrics.KnownMessages.Load()))
		gossipPeersMessages.With(getLabels("sent")).Set(float64(gossipProto.Metrics.SentMessages.Load()))

		gossipPeersRequests.With(getLabels("received_message")).Set(float64(gossipProto.Metrics.ReceivedMessageRequests.Load()))
		gossipPeersRequests.With(getLabels("received_milestone")).Set(float64(gossipProto.Metrics.ReceivedMilestoneRequests.Load()))
		gossipPeersRequests.With(getLabels("sent_message")).Set(float64(gossipProto.Metrics.SentMessageRequests.Load()))
		gossipPeersRequests.With(getLabels("sent_milestone")).Set(float64(gossipProto.Metrics.SentMilestoneRequests.Load()))

		gossipPeersHeartbeats.With(getLabels("received")).Set(float64(gossipProto.Metrics.ReceivedHeartbeats.Load()))
		gossipPeersHeartbeats.With(getLabels("sent")).Set(float64(gossipProto.Metrics.SentHeartbeats.Load()))

		gossipPeersDroppedPackets.With(getLabels("sent")).Set(float64(peer.DroppedSentPackets))

		gossipPeersConnected.With(peerLabels).Set(0)
		if peer.Connected {
			gossipPeersConnected.With(peerLabels).Set(1)
		}
	}
}
