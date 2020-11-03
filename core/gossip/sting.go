package gossip

import (
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/events"
)

// sets up the event handlers which propagate STING messages.
func addMessageEventHandlers(proto *gossip.Protocol) {
	msgProc := MessageProcessor()

	proto.Parser.Events.Received[gossip.MessageTypeMessage].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMessages.Inc()
		metrics.SharedServerMetrics.Messages.Inc()
		msgProc.Process(proto, gossip.MessageTypeMessage, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMessage].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMessages.Inc()
		metrics.SharedServerMetrics.SentMessages.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMessageRequest].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMessageRequests.Inc()
		metrics.SharedServerMetrics.ReceivedMessageRequests.Inc()
		msgProc.Process(proto, gossip.MessageTypeMessageRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMessageRequest].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMessageRequests.Inc()
		metrics.SharedServerMetrics.SentMessageRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMilestoneRequest].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMilestoneRequests.Inc()
		metrics.SharedServerMetrics.ReceivedMilestoneRequests.Inc()
		msgProc.Process(proto, gossip.MessageTypeMilestoneRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMilestoneRequest].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMilestoneRequests.Inc()
		metrics.SharedServerMetrics.SentMilestoneRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedHeartbeats.Inc()
		metrics.SharedServerMetrics.ReceivedHeartbeats.Inc()

		proto.LatestHeartbeat = gossip.ParseHeartbeat(data)

		/*
			if p.Autopeering != nil && p.LatestHeartbeat.SolidMilestoneIndex < tangle.GetSnapshotInfo().PruningIndex {
				// peer is connected via autopeering and its latest solid milestone index is below our pruning index.
				// we can't help this neighbor to become sync, so it's better to drop the connection and free the slots for other peers.
				log.Infof("dropping autopeered neighbor %s / %s because LSMI (%d) is below our pruning index (%d)", p.Autopeering.Address(), p.Autopeering.ID(), p.LatestHeartbeat.SolidMilestoneIndex, tangle.GetSnapshotInfo().PruningIndex)
				peering.Manager().Remove(p.ID)
				return
			}
		*/

		proto.HeartbeatReceivedTime = time.Now()
		proto.Events.HeartbeatUpdated.Trigger(proto.LatestHeartbeat)
	}))

	proto.Events.Sent[gossip.MessageTypeHeartbeat].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentHeartbeats.Inc()
		metrics.SharedServerMetrics.SentHeartbeats.Inc()
		proto.HeartbeatSentTime = time.Now()
	}))
}

// removeMessageEventHandlers removes all the event handlers for sent and received messages.
func removeMessageEventHandlers(proto *gossip.Protocol) {
	if proto == nil {
		return
	}

	if proto.Parser.Events.Received != nil {
		for _, event := range proto.Parser.Events.Received {
			if event == nil {
				continue
			}
			event.DetachAll()
		}
	}

	if proto.Events.Sent != nil {
		for _, event := range proto.Events.Sent {
			if event == nil {
				continue
			}
			event.DetachAll()
		}
	}
}
