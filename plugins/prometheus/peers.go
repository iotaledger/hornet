package prometheus

import (
	"net"

	"github.com/gohornet/hornet/plugins/peering"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	peersAllTransactions             *prometheus.GaugeVec
	peersNewTransactions             *prometheus.GaugeVec
	peersKnownTransactions           *prometheus.GaugeVec
	peersStaleTransactions           *prometheus.GaugeVec
	peersReceivedTransactionRequests *prometheus.GaugeVec
	peersReceivedMilestoneRequests   *prometheus.GaugeVec
	peersReceivedHeartbeats          *prometheus.GaugeVec
	peersSentTransactions            *prometheus.GaugeVec
	peersSentTransactionRequests     *prometheus.GaugeVec
	peersSentMilestoneRequests       *prometheus.GaugeVec
	peersSentHeartbeats              *prometheus.GaugeVec
	peersDroppedSentPackets          *prometheus.GaugeVec
	peersConnected                   *prometheus.GaugeVec
)

func init() {
	peersAllTransactions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_all_transactions",
			Help: "Number of all transactions by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersNewTransactions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_new_transactions",
			Help: "Number of new transactions by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersKnownTransactions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_known_transactions",
			Help: "Number of known transactions by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersStaleTransactions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_stale_transactions",
			Help: "Number of stale transactions by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersReceivedTransactionRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_received_transaction_requests",
			Help: "Number of received transaction requests by peer.",
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
	peersSentTransactions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_sent_transactions",
			Help: "Number of sent transactions by peer.",
		},
		[]string{"address", "port", "domain", "alias", "type", "autopeering_id"},
	)
	peersSentTransactionRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_peers_sent_transaction_requests",
			Help: "Number of sent transaction requests by peer.",
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

	registry.MustRegister(peersAllTransactions)
	registry.MustRegister(peersNewTransactions)
	registry.MustRegister(peersKnownTransactions)
	registry.MustRegister(peersStaleTransactions)
	registry.MustRegister(peersReceivedTransactionRequests)
	registry.MustRegister(peersReceivedMilestoneRequests)
	registry.MustRegister(peersReceivedHeartbeats)
	registry.MustRegister(peersSentTransactions)
	registry.MustRegister(peersSentTransactionRequests)
	registry.MustRegister(peersSentMilestoneRequests)
	registry.MustRegister(peersSentHeartbeats)
	registry.MustRegister(peersDroppedSentPackets)
	registry.MustRegister(peersConnected)

	addCollect(collectPeers)
}

func collectPeers() {
	peersAllTransactions.Reset()
	peersNewTransactions.Reset()
	peersKnownTransactions.Reset()
	peersStaleTransactions.Reset()
	peersReceivedTransactionRequests.Reset()
	peersReceivedMilestoneRequests.Reset()
	peersReceivedHeartbeats.Reset()
	peersSentTransactions.Reset()
	peersSentTransactionRequests.Reset()
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

		peersAllTransactions.With(labels).Set(float64(peer.NumberOfAllTransactions))
		peersNewTransactions.With(labels).Set(float64(peer.NumberOfNewTransactions))
		peersKnownTransactions.With(labels).Set(float64(peer.NumberOfKnownTransactions))
		peersStaleTransactions.With(labels).Set(float64(peer.NumberOfStaleTransactions))
		peersReceivedTransactionRequests.With(labels).Set(float64(peer.NumberOfReceivedTransactionReq))
		peersReceivedMilestoneRequests.With(labels).Set(float64(peer.NumberOfReceivedMilestoneReq))
		peersReceivedHeartbeats.With(labels).Set(float64(peer.NumberOfReceivedHeartbeats))
		peersSentTransactions.With(labels).Set(float64(peer.NumberOfSentTransactions))
		peersSentTransactionRequests.With(labels).Set(float64(peer.NumberOfSentTransactionsReq))
		peersSentMilestoneRequests.With(labels).Set(float64(peer.NumberOfSentMilestoneReq))
		peersSentHeartbeats.With(labels).Set(float64(peer.NumberOfSentHeartbeats))
		peersDroppedSentPackets.With(labels).Set(float64(peer.NumberOfDroppedSentPackets))
		peersConnected.With(labels).Set(0)
		if peer.Connected {
			peersConnected.With(labels).Set(1)
		}
	}
}
