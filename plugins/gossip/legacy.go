package gossip

import (
	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/peering/peer"
	"github.com/gohornet/hornet/packages/protocol/legacy"
	"github.com/iotaledger/hive.go/events"
)

// sets up the event handlers which propagate legacy messages.
func addLegacyMessageEventHandlers(p *peer.Peer) {
	p.Protocol.Events.Received[legacy.TransactionAndRequestMessageDefinition.ID].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedTransactions.Inc()
		metrics.SharedServerMetrics.Transactions.Inc()
		msgProcessor.Process(p, legacy.TransactionAndRequestMessageDefinition.ID, data)
	}))

	p.Protocol.Events.Sent[legacy.TransactionAndRequestMessageDefinition.ID].Attach(events.NewClosure(func() {
		p.Metrics.SentTransactions.Inc()
	}))
}
