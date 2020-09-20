package prometheus

import (
	"net"

	"github.com/gohornet/hornet/plugins/peering"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	peersAllMessages               *prometheus.GaugeVec
	peersNewMessages               *prometheus.GaugeVec
	peersKnownMessages             *prometheus.GaugeVec
	peersReceivedMessageRequests   *prometheus.GaugeVec
	peersReceivedMilestoneRequests *prometheus.GaugeVec
	peersReceivedHeartbeats        *prometheus.GaugeVec
	peersSentMessages              *prometheus.GaugeVec
	peersSentMessageRequests       *prometheus.GaugeVec
	peersSentMilestoneRequests     *prometheus.GaugeVec
	peersSentHeartbeats            *prometheus.GaugeVec
	peersDroppedSentPackets        *prometheus.GaugeVec
	peersConnected                 *prometheus.GaugeVec
)

func init() {
	peersAllMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_all_messages",
			Help: "Number of all messages by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersNewMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_new_messages",
			Help: "Number of new messages by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersKnownMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_known_messages",
			Help: "Number of known messages by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersReceivedMessageRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_received_message_requests",
			Help: "Number of received message requests by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersReceivedMilestoneRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_received_milestone_requests",
			Help: "Number of received milestone requests by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersReceivedHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_received_heartbeats",
			Help: "Number of received heartbeats by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersSentMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_sent_messages",
			Help: "Number of sent messages by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersSentMessageRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_sent_message_requests",
			Help: "Number of sent message requests by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersSentMilestoneRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_sent_milestone_requests",
			Help: "Number of sent milestone requests by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersSentHeartbeats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_sent_heartbeats",
			Help: "Number of sent heartbeats by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersDroppedSentPackets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_dropped_sent_packets",
			Help: "Number of dropped sent packets by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersConnected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_connected",
			Help: "Are peers connected?",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)

	registry.MustRegister(peersAllMessages)
	registry.MustRegister(peersNewMessages)
	registry.MustRegister(peersKnownMessages)
	registry.MustRegister(peersReceivedMessageRequests)
	registry.MustRegister(peersReceivedMilestoneRequests)
	registry.MustRegister(peersReceivedHeartbeats)
	registry.MustRegister(peersSentMessages)
	registry.MustRegister(peersSentMessageRequests)
	registry.MustRegister(peersSentMilestoneRequests)
	registry.MustRegister(peersSentHeartbeats)
	registry.MustRegister(peersDroppedSentPackets)
	registry.MustRegister(peersConnected)

	addCollect(collectPeers)
}

func collectPeers() {
	peersAllMessages.Reset()
	peersNewMessages.Reset()
	peersKnownMessages.Reset()
	peersReceivedMessageRequests.Reset()
	peersReceivedMilestoneRequests.Reset()
	peersReceivedHeartbeats.Reset()
	peersSentMessages.Reset()
	peersSentMessageRequests.Reset()
	peersSentMilestoneRequests.Reset()
	peersSentHeartbeats.Reset()
	peersDroppedSentPackets.Reset()
	peersConnected.Reset()

	for _, peer := range peering.Manager().PeerInfos() {
		address, port, _ := net.SplitHostPort(peer.Address)
		labels := prometheus.Labels{
			"address":        address,
			"port":           port,
			"domain":         peer.Domain,
			"alias":          peer.Alias,
			"type":           peer.ConnectionType,
			"autopeering_id": peer.AutopeeringID,
		}

		peersAllMessages.With(labels).Set(float64(peer.ReceivedMessages))
		peersNewMessages.With(labels).Set(float64(peer.NewMessages))
		peersKnownMessages.With(labels).Set(float64(peer.KnownMessages))
		peersReceivedMessageRequests.With(labels).Set(float64(peer.ReceivedMessageReq))
		peersReceivedMilestoneRequests.With(labels).Set(float64(peer.ReceivedMilestoneReq))
		peersReceivedHeartbeats.With(labels).Set(float64(peer.ReceivedHeartbeats))
		peersSentMessages.With(labels).Set(float64(peer.SentMessages))
		peersSentMessageRequests.With(labels).Set(float64(peer.SentMessageReq))
		peersSentMilestoneRequests.With(labels).Set(float64(peer.SentMilestoneReq))
		peersSentHeartbeats.With(labels).Set(float64(peer.SentHeartbeats))
		peersDroppedSentPackets.With(labels).Set(float64(peer.DroppedSentPackets))
		peersConnected.With(labels).Set(0)
		if peer.Connected {
			peersConnected.With(labels).Set(1)
		}
	}
}
