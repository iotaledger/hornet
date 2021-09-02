package gossip

import (
	"time"

	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/events"
)

// sets up the event handlers which propagate STING messages.
func addMessageEventHandlers(proto *gossip.Protocol) {

	proto.Parser.Events.Received[gossip.MessageTypeMessage].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMessages.Inc()
		deps.ServerMetrics.Messages.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMessage, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMessage].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMessages.Inc()
		deps.ServerMetrics.SentMessages.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMessageRequest].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMessageRequests.Inc()
		deps.ServerMetrics.ReceivedMessageRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMessageRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMessageRequest].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMessageRequests.Inc()
		deps.ServerMetrics.SentMessageRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMilestoneRequest].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMilestoneRequests.Inc()
		deps.ServerMetrics.ReceivedMilestoneRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMilestoneRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMilestoneRequest].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMilestoneRequests.Inc()
		deps.ServerMetrics.SentMilestoneRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedHeartbeats.Inc()
		deps.ServerMetrics.ReceivedHeartbeats.Inc()

		proto.LatestHeartbeat = gossip.ParseHeartbeat(data)

		/*
			if p.Autopeering != nil && p.LatestHeartbeat.SolidMilestoneIndex < tangle.SnapshotInfo().PruningIndex {
				// peer is connected via autopeering and its solid milestone index is below our pruning index.
				// we can't help this neighbor to become sync, so it's better to drop the connection and free the slots for other peers.
				log.Infof("dropping autopeered neighbor %s / %s because SMI (%d) is below our pruning index (%d)", p.Autopeering.Address(), p.Autopeering.ID(), p.LatestHeartbeat.SolidMilestoneIndex, tangle.SnapshotInfo().PruningIndex)
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
		deps.ServerMetrics.SentHeartbeats.Inc()
		proto.HeartbeatSentTime = time.Now()
	}))
}

// ToDo: re-add this function and check why it deadlocks
//nolint:deadcode,unused // should be added again later
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
