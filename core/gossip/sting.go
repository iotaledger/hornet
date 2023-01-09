package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/protocol/gossip"
)

// sets up the event handlers which propagate STING messages.
func attachEventsProtocolMessages(proto *gossip.Protocol) {

	proto.Parser.Events.Received[gossip.MessageTypeMessage].Hook(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMessages.Inc()
		deps.ServerMetrics.Messages.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMessage, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMessage].Hook(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMessages.Inc()
		deps.ServerMetrics.SentMessages.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMessageRequest].Hook(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMessageRequests.Inc()
		deps.ServerMetrics.ReceivedMessageRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMessageRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMessageRequest].Hook(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMessageRequests.Inc()
		deps.ServerMetrics.SentMessageRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeMilestoneRequest].Hook(events.NewClosure(func(data []byte) {
		proto.Metrics.ReceivedMilestoneRequests.Inc()
		deps.ServerMetrics.ReceivedMilestoneRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMilestoneRequest, data)
	}))

	proto.Events.Sent[gossip.MessageTypeMilestoneRequest].Hook(events.NewClosure(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMilestoneRequests.Inc()
		deps.ServerMetrics.SentMilestoneRequests.Inc()
	}))

	proto.Parser.Events.Received[gossip.MessageTypeHeartbeat].Hook(events.NewClosure(func(data []byte) {
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
