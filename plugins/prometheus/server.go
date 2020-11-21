package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	serverAllMessages               prometheus.Gauge
	serverNewMessages               prometheus.Gauge
	serverKnownMessages             prometheus.Gauge
	serverReferencedMessages        prometheus.Gauge
	serverInvalidMessages           prometheus.Gauge
	serverInvalidRequests           prometheus.Gauge
	serverReceivedMessageRequests   prometheus.Gauge
	serverReceivedMilestoneRequests prometheus.Gauge
	serverReceivedHeartbeats        prometheus.Gauge
	serverSentMessages              prometheus.Gauge
	serverSentMessageRequests       prometheus.Gauge
	serverSentMilestoneRequests     prometheus.Gauge
	serverSentHeartbeats            prometheus.Gauge
	serverDroppedSentPackets        prometheus.Gauge
	serverSentSpamMessages          prometheus.Gauge
	serverValidatedMessages         prometheus.Gauge
)

func configureServer() {
	serverAllMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_all_messages",
		Help: "Number of all messages.",
	})
	serverNewMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_new_messages",
		Help: "Number of new messages.",
	})
	serverKnownMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_known_messages",
		Help: "Number of known messages.",
	})
	serverReferencedMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_referenced_messages",
		Help: "Number of referenced messages.",
	})
	serverInvalidMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_invalid_messages",
		Help: "Number of invalid messages.",
	})
	serverInvalidRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_invalid_requests",
		Help: "Number of invalid requests.",
	})
	serverReceivedMessageRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_received_message_requests",
		Help: "Number of received message requests.",
	})
	serverReceivedMilestoneRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_received_milestone_requests",
		Help: "Number of received milestone requests.",
	})
	serverReceivedHeartbeats = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_received_heartbeats",
		Help: "Number of received heartbeats.",
	})
	serverSentMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_messages",
		Help: "Number of sent messages.",
	})
	serverSentMessageRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_message_requests",
		Help: "Number of sent message requests.",
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
	serverSentSpamMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_sent_spam_messages",
		Help: "Number of sent spam messages.",
	})
	serverValidatedMessages = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_server_validated_messages",
		Help: "Number of validated messages.",
	})

	registry.MustRegister(serverAllMessages)
	registry.MustRegister(serverNewMessages)
	registry.MustRegister(serverKnownMessages)
	registry.MustRegister(serverReferencedMessages)
	registry.MustRegister(serverInvalidMessages)
	registry.MustRegister(serverInvalidRequests)
	registry.MustRegister(serverReceivedMessageRequests)
	registry.MustRegister(serverReceivedMilestoneRequests)
	registry.MustRegister(serverReceivedHeartbeats)
	registry.MustRegister(serverSentMessages)
	registry.MustRegister(serverSentMessageRequests)
	registry.MustRegister(serverSentMilestoneRequests)
	registry.MustRegister(serverSentHeartbeats)
	registry.MustRegister(serverDroppedSentPackets)
	registry.MustRegister(serverSentSpamMessages)
	registry.MustRegister(serverValidatedMessages)

	addCollect(collectServer)
}

func collectServer() {
	serverAllMessages.Set(float64(deps.ServerMetrics.Messages.Load()))
	serverNewMessages.Set(float64(deps.ServerMetrics.NewMessages.Load()))
	serverKnownMessages.Set(float64(deps.ServerMetrics.KnownMessages.Load()))
	serverReferencedMessages.Set(float64(deps.ServerMetrics.ReferencedMessages.Load()))
	serverInvalidMessages.Set(float64(deps.ServerMetrics.InvalidMessages.Load()))
	serverInvalidRequests.Set(float64(deps.ServerMetrics.InvalidRequests.Load()))
	serverReceivedMessageRequests.Set(float64(deps.ServerMetrics.ReceivedMessageRequests.Load()))
	serverReceivedMilestoneRequests.Set(float64(deps.ServerMetrics.ReceivedMilestoneRequests.Load()))
	serverReceivedHeartbeats.Set(float64(deps.ServerMetrics.ReceivedHeartbeats.Load()))
	serverSentMessages.Set(float64(deps.ServerMetrics.SentMessages.Load()))
	serverSentMessageRequests.Set(float64(deps.ServerMetrics.SentMessageRequests.Load()))
	serverSentMilestoneRequests.Set(float64(deps.ServerMetrics.SentMilestoneRequests.Load()))
	serverSentHeartbeats.Set(float64(deps.ServerMetrics.SentHeartbeats.Load()))
	serverDroppedSentPackets.Set(float64(deps.ServerMetrics.DroppedMessages.Load()))
	serverSentSpamMessages.Set(float64(deps.ServerMetrics.SentSpamMessages.Load()))
	serverValidatedMessages.Set(float64(deps.ServerMetrics.ValidatedMessages.Load()))
}
