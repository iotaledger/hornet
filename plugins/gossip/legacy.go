package gossip

import (
	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/peering/peer"
	"github.com/gohornet/hornet/packages/protocol/legacy"
	"github.com/iotaledger/hive.go/events"
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
