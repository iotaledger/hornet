package gossip

import (
	"time"

	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
)

// sets up the event handlers which propagate STING messages.
func hookEventsProtocolMessages(proto *gossip.Protocol) {
	proto.Parser.Events.Received[gossip.MessageTypeBlock].Hook(func(data []byte) {
		proto.Metrics.ReceivedBlocks.Inc()
		deps.ServerMetrics.Blocks.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeBlock, data)
	})

	proto.Events.Sent[gossip.MessageTypeBlock].Hook(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentBlocks.Inc()
		deps.ServerMetrics.SentBlocks.Inc()
	})

	proto.Parser.Events.Received[gossip.MessageTypeBlockRequest].Hook(func(data []byte) {
		proto.Metrics.ReceivedBlockRequests.Inc()
		deps.ServerMetrics.ReceivedBlockRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeBlockRequest, data)
	})

	proto.Events.Sent[gossip.MessageTypeBlockRequest].Hook(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentBlockRequests.Inc()
		deps.ServerMetrics.SentBlockRequests.Inc()
	})

	proto.Parser.Events.Received[gossip.MessageTypeMilestoneRequest].Hook(func(data []byte) {
		proto.Metrics.ReceivedMilestoneRequests.Inc()
		deps.ServerMetrics.ReceivedMilestoneRequests.Inc()
		deps.MessageProcessor.Process(proto, gossip.MessageTypeMilestoneRequest, data)
	})

	proto.Events.Sent[gossip.MessageTypeMilestoneRequest].Hook(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentMilestoneRequests.Inc()
		deps.ServerMetrics.SentMilestoneRequests.Inc()
	})

	proto.Parser.Events.Received[gossip.MessageTypeHeartbeat].Hook(func(data []byte) {
		proto.Metrics.ReceivedHeartbeats.Inc()
		deps.ServerMetrics.ReceivedHeartbeats.Inc()

		proto.LatestHeartbeat = gossip.ParseHeartbeat(data)
		proto.HeartbeatReceivedTime = time.Now()
		proto.Events.HeartbeatUpdated.Trigger(proto.LatestHeartbeat)
	})

	proto.Events.Sent[gossip.MessageTypeHeartbeat].Hook(func() {
		proto.Metrics.SentPackets.Inc()
		proto.Metrics.SentHeartbeats.Inc()
		deps.ServerMetrics.SentHeartbeats.Inc()
		proto.HeartbeatSentTime = time.Now()
	})
}
