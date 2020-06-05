package gossip

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/legacy"
)

// sets up the event handlers which propagate legacy messages.
func addLegacyMessageEventHandlers(p *peer.Peer) {
	p.Protocol.Events.Received[legacy.MessageTypeTransactionAndRequest].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedTransactions.Inc()
		metrics.SharedServerMetrics.Transactions.Inc()
		msgProcessor.Process(p, legacy.MessageTypeTransactionAndRequest, data)
	}))

	p.Protocol.Events.Sent[legacy.MessageTypeTransactionAndRequest].Attach(events.NewClosure(func() {
		p.Metrics.SentTransactions.Inc()
		metrics.SharedServerMetrics.SentTransactions.Inc()
	}))
}
