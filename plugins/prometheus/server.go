package prometheus

import (
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	serverAllTransactions             prometheus.Gauge
	serverNewTransactions             prometheus.Gauge
	serverKnownTransactions           prometheus.Gauge
	serverConfirmedTransactions       prometheus.Gauge
	serverInvalidTransactions         prometheus.Gauge
	serverInvalidRequests             prometheus.Gauge
	serverStaleTransactions           prometheus.Gauge
	serverReceivedTransactionRequests prometheus.Gauge
	serverReceivedMilestoneRequests   prometheus.Gauge
	serverReceivedHeartbeats          prometheus.Gauge
	serverSentTransactions            prometheus.Gauge
	serverSentTransactionRequests     prometheus.Gauge
	serverSentMilestoneRequests       prometheus.Gauge
	serverSentHeartbeats              prometheus.Gauge
	serverDroppedSentPackets          prometheus.Gauge
	serverSentSpamTransactions        prometheus.Gauge
	serverValidatedBundles            prometheus.Gauge
	serverSeenSpentAddresses          prometheus.Gauge
)

func init() {
	serverAllTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_all_transactions",
		Help: "Number of all transactions.",
	})
	serverNewTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_new_transactions",
		Help: "Number of new transactions.",
	})
	serverKnownTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_known_transactions",
		Help: "Number of known transactions.",
	})
	serverConfirmedTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_confirmed_transactions",
		Help: "Number of confirmed transactions.",
	})
	serverInvalidTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_invalid_transactions",
		Help: "Number of invalid transactions.",
	})
	serverInvalidRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_invalid_requests",
		Help: "Number of invalid requests.",
	})
	serverStaleTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_stale_transactions",
		Help: "Number of stale transactions.",
	})
	serverReceivedTransactionRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_received_transaction_requests",
		Help: "Number of received transaction requests.",
	})
	serverReceivedMilestoneRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_received_milestone_requests",
		Help: "Number of received milestone requests.",
	})
	serverReceivedHeartbeats = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_received_heartbeats",
		Help: "Number of received heartbeats.",
	})
	serverSentTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_transactions",
		Help: "Number of sent transactions.",
	})
	serverSentTransactionRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_transaction_requests",
		Help: "Number of sent transaction requests.",
	})
	serverSentMilestoneRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_milestone_requests",
		Help: "Number of sent milestone requests.",
	})
	serverSentHeartbeats = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_heartbeats",
		Help: "Number of sent heartbeats.",
	})
	serverDroppedSentPackets = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_dropped_sent_packets",
		Help: "Number of dropped sent packets.",
	})
	serverSentSpamTransactions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_spam_transactions",
		Help: "Number of sent spam transactions.",
	})
	serverValidatedBundles = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_validated_bundles",
		Help: "Number of validated bundles.",
	})
	serverSeenSpentAddresses = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_seen_spent_addresses",
		Help: "Number of seen spent addresses.",
	})

	registry.MustRegister(serverAllTransactions)
	registry.MustRegister(serverNewTransactions)
	registry.MustRegister(serverKnownTransactions)
	registry.MustRegister(serverConfirmedTransactions)
	registry.MustRegister(serverInvalidTransactions)
	registry.MustRegister(serverInvalidRequests)
	registry.MustRegister(serverStaleTransactions)
	registry.MustRegister(serverReceivedTransactionRequests)
	registry.MustRegister(serverReceivedMilestoneRequests)
	registry.MustRegister(serverReceivedHeartbeats)
	registry.MustRegister(serverSentTransactions)
	registry.MustRegister(serverSentTransactionRequests)
	registry.MustRegister(serverSentMilestoneRequests)
	registry.MustRegister(serverSentHeartbeats)
	registry.MustRegister(serverDroppedSentPackets)
	registry.MustRegister(serverSentSpamTransactions)
	registry.MustRegister(serverValidatedBundles)
	registry.MustRegister(serverSeenSpentAddresses)

	addCollect(collectServer)
}

func collectServer() {
	serverAllTransactions.Set(float64(metrics.SharedServerMetrics.Transactions.Load()))
	serverNewTransactions.Set(float64(metrics.SharedServerMetrics.NewTransactions.Load()))
	serverKnownTransactions.Set(float64(metrics.SharedServerMetrics.KnownTransactions.Load()))
	serverConfirmedTransactions.Set(float64(metrics.SharedServerMetrics.ConfirmedTransactions.Load()))
	serverInvalidTransactions.Set(float64(metrics.SharedServerMetrics.InvalidTransactions.Load()))
	serverInvalidRequests.Set(float64(metrics.SharedServerMetrics.InvalidRequests.Load()))
	serverStaleTransactions.Set(float64(metrics.SharedServerMetrics.StaleTransactions.Load()))
	serverReceivedTransactionRequests.Set(float64(metrics.SharedServerMetrics.ReceivedTransactionRequests.Load()))
	serverReceivedMilestoneRequests.Set(float64(metrics.SharedServerMetrics.ReceivedMilestoneRequests.Load()))
	serverReceivedHeartbeats.Set(float64(metrics.SharedServerMetrics.ReceivedHeartbeats.Load()))
	serverSentTransactions.Set(float64(metrics.SharedServerMetrics.SentTransactions.Load()))
	serverSentTransactionRequests.Set(float64(metrics.SharedServerMetrics.SentTransactionRequests.Load()))
	serverSentMilestoneRequests.Set(float64(metrics.SharedServerMetrics.SentMilestoneRequests.Load()))
	serverSentHeartbeats.Set(float64(metrics.SharedServerMetrics.SentHeartbeats.Load()))
	serverDroppedSentPackets.Set(float64(metrics.SharedServerMetrics.DroppedMessages.Load()))
	serverSentSpamTransactions.Set(float64(metrics.SharedServerMetrics.SentSpamTransactions.Load()))
	serverValidatedBundles.Set(float64(metrics.SharedServerMetrics.ValidatedBundles.Load()))
	serverSeenSpentAddresses.Set(float64(metrics.SharedServerMetrics.SeenSpentAddresses.Load()))
}
