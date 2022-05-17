package gossip

import (
	"time"

	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/events"
)

// sets up the event handlers which propagate STING messages.
func attachEventsProtocolMessages(proto *gossip.Protocol) {

	proto.Parser.Events.Received[gossip.MessageTypeBlock].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedBlocks.Inc()
		deps.ServerMetrics.Blocks.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeBlock, data)
	}))

	proto.Events.Sent[gossip.MessageTypeBlock].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentBlocks.Inc()
		deps.ServerMetrics.SentBlocks.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeBlockRequest].Attach(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedBlockRequests.Inc()
		deps.ServerMetrics.ReceivedBlockRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeBlockRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeBlockRequest].Attach(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentBlockRequests.Inc()
		deps.ServerMetrics.SentBlockRequests.Inc()
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
			// TODO: reintroduce
			if proto.Autopeering != nil && p.LatestHeartbeat.SolidMilestoneIndex < tangle.SnapshotInfo().PruningIndex {
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

// detachEventsProtocolMessages removes all the event handlers for sent and received messages.
func detachEventsProtocolMessages(proto *gossip.Protocol) {
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
