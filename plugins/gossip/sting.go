package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/plugins/peering"
)

// sets up the event handlers which propagate STING messages.
func addSTINGMessageEventHandlers(p *peer.Peer) {

	p.Protocol.Events.Received[sting.MessageTypeTransaction].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedTransactions.Inc()
		metrics.SharedServerMetrics.Transactions.Inc()
		msgProcessor.Process(p, sting.MessageTypeTransaction, data)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeTransaction].Attach(events.NewClosure(func() {
		p.Metrics.SentPackets.Inc()
		p.Metrics.SentTransactions.Inc()
		metrics.SharedServerMetrics.SentTransactions.Inc()
	}))

	p.Protocol.Events.Received[sting.MessageTypeTransactionRequest].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedTransactionRequests.Inc()
		metrics.SharedServerMetrics.ReceivedTransactionRequests.Inc()
		msgProcessor.Process(p, sting.MessageTypeTransactionRequest, data)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeTransactionRequest].Attach(events.NewClosure(func() {
		p.Metrics.SentPackets.Inc()
		p.Metrics.SentTransactionRequests.Inc()
		metrics.SharedServerMetrics.SentTransactionRequests.Inc()
	}))

	p.Protocol.Events.Received[sting.MessageTypeMilestoneRequest].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedMilestoneRequests.Inc()
		metrics.SharedServerMetrics.ReceivedMilestoneRequests.Inc()
		msgProcessor.Process(p, sting.MessageTypeMilestoneRequest, data)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeMilestoneRequest].Attach(events.NewClosure(func() {
		p.Metrics.SentPackets.Inc()
		p.Metrics.SentMilestoneRequests.Inc()
		metrics.SharedServerMetrics.SentMilestoneRequests.Inc()
	}))

	p.Protocol.Events.Received[sting.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
		p.Metrics.ReceivedHeartbeats.Inc()
		metrics.SharedServerMetrics.ReceivedHeartbeats.Inc()

		p.LatestHeartbeat = sting.ParseHeartbeat(data)
		p.HeartbeatReceivedTime = time.Now()

		if p.Autopeering != nil && p.LatestHeartbeat.SolidMilestoneIndex < tangle.GetSnapshotInfo().PruningIndex {
			// peer is connected via autopeering and its latest solid milestone index is below our pruning index.
			// we can't help this neighbor to become sync, so it's better to drop the connection and free the slots for other peers.
			log.Infof("dropping autopeered neighbor %s / %s because LSMI (%d) is below our pruning index (%d)", p.Autopeering.Address(), p.Autopeering.ID(), p.LatestHeartbeat.SolidMilestoneIndex, tangle.GetSnapshotInfo().PruningIndex)
			peering.Manager().Remove(p.ID)
			return
		}

		p.Events.HeartbeatUpdated.Trigger(p.LatestHeartbeat)
	}))

	p.Protocol.Events.Sent[sting.MessageTypeHeartbeat].Attach(events.NewClosure(func() {
		p.Metrics.SentPackets.Inc()
		p.Metrics.SentHeartbeats.Inc()
		metrics.SharedServerMetrics.SentHeartbeats.Inc()

		p.HeartbeatSentTime = time.Now()
	}))
}

// removeMessageEventHandlers removes all the event handlers for sent and received messages.
func removeMessageEventHandlers(p *peer.Peer) {

	if (p == nil) || (p.Protocol == nil) {
		return
	}

	if p.Protocol.Events.Received != nil {
		for _, event := range p.Protocol.Events.Received {
			if event == nil {
				continue
			}
			event.DetachAll()
		}
	}

	if p.Protocol.Events.Sent != nil {
		for _, event := range p.Protocol.Events.Sent {
			if event == nil {
				continue
			}
			event.DetachAll()
		}
	}
}
