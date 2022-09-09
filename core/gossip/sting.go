package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
)

// sets up the event handlers which propagate STING messages.
func attachEventsProtocolMessages(proto *gossip.Protocol) {

	proto.Parser.Events.Received[gossip.MessageTypeBlock].Hook(event.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedBlocks.Inc()
		deps.ServerMetrics.Blocks.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeBlock, data)
	}))

	proto.Events.Sent[gossip.MessageTypeBlock].Hook(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentBlocks.Inc()
		deps.ServerMetrics.SentBlocks.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeBlockRequest].Hook(event.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedBlockRequests.Inc()
		deps.ServerMetrics.ReceivedBlockRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeBlockRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeBlockRequest].Hook(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentBlockRequests.Inc()
		deps.ServerMetrics.SentBlockRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMilestoneRequest].Hook(event.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMilestoneRequests.Inc()
		deps.ServerMetrics.ReceivedMilestoneRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMilestoneRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMilestoneRequest].Hook(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMilestoneRequests.Inc()
		deps.ServerMetrics.SentMilestoneRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeHeartbeat].Hook(event.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedHeartbeats.Inc()
		deps.ServerMetrics.ReceivedHeartbeats.Inc()

		proto.LatestHeartbeat = gossip.ParseHeartbeat(data)
		proto.HeartbeatReceivedTime = time.Now()
		proto.Events.HeartbeatUpdated.Trigger(proto.LatestHeartbeat)
	}))

	proto.Events.Sent[gossip.MessageTypeHeartbeat].Hook(events.NewClosure(func() {
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
