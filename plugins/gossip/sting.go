package gossip

import (
	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/peering/peer"
	"github.com/gohornet/hornet/packages/protocol/helpers"
	"github.com/gohornet/hornet/packages/protocol/sting"
	"github.com/iotaledger/hive.go/events"
)

// sets up the event handlers which propagate STING messages.
func addSTINGMessageEventHandlers(p *peer.Peer) {

	p.Protocol.Events.Received[sting.MessageTypeTransaction].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedTransactions.Inc()
		metrics.SharedServerMetrics.Transactions.Inc()
		msgProcessor.Process(p, sting.MessageTypeTransaction, data)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeTransaction].Attach(events.NewClosure(func() {
		p.Metrics.SentTransactions.Inc()
		metrics.SharedServerMetrics.SentTransactions.Inc()
	}))

	p.Protocol.Events.Received[sting.MessageTypeTransactionRequest].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedTransactionRequests.Inc()
		msgProcessor.Process(p, sting.MessageTypeTransactionRequest, data)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeTransactionRequest].Attach(events.NewClosure(func() {
		p.Metrics.SentTransactionRequests.Inc()
		metrics.SharedServerMetrics.SentTransactionRequests.Inc()
	}))

	p.Protocol.Events.Received[sting.MessageTypeMilestoneRequest].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedMilestoneRequests.Inc()
		msgProcessor.Process(p, sting.MessageTypeMilestoneRequest, data)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeMilestoneRequest].Attach(events.NewClosure(func() {
		p.Metrics.SentMilestoneRequests.Inc()
		metrics.SharedServerMetrics.SentMilestoneRequests.Inc()
	}))

	p.Protocol.Events.Received[sting.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedHeartbeats.Inc()

		// update heartbeat
		firstHeartbeat := p.LatestHeartbeat == nil
		p.LatestHeartbeat = sting.ParseHeartbeat(data)

		// set the first heartbeat
		if firstHeartbeat {
			if snapshotInfo := tangle.GetSnapshotInfo(); snapshotInfo != nil {
				helpers.SendHeartbeat(p, tangle.GetSolidMilestoneIndex(), snapshotInfo.PruningIndex)
				helpers.SendLatestMilestoneRequest(p)
			}
		}
	}))

	p.Protocol.Events.Sent[sting.MessageTypeHeartbeat].Attach(events.NewClosure(func() {
		p.Metrics.SentHeartbeats.Inc()
		metrics.SharedServerMetrics.SentHeartbeats.Inc()
	}))
}
