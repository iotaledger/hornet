package gossip

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
)

var (
	gossipProtocolHooks      = shrinkingmap.New[peer.ID, func()]()
	gossipProtocolHooksMutex sync.Mutex
)

// sets up the event handlers which propagate STING messages.
func hookGossipProtocolEvents(proto *gossip.Protocol) {
	gossipProtocolHooksMutex.Lock()
	defer gossipProtocolHooksMutex.Unlock()

	if unhook, exists := gossipProtocolHooks.Get(proto.PeerID); exists {
		unhook()
		gossipProtocolHooks.Delete(proto.PeerID)
	}

	// attach protocol errors
	closeConnectionDueToProtocolError := func(err error) {
		Component.LogWarnf("closing connection to peer %s because of a protocol error: %s", proto.PeerID.ShortString(), err.Error())

		if err := deps.GossipService.CloseStream(proto.PeerID); err != nil {
			Component.LogWarnf("closing connection to peer %s failed, error: %s", proto.PeerID.ShortString(), err.Error())
		}
	}

	gossipProtocolHooks.Set(proto.PeerID, lo.Batch(
		proto.Events.Errors.Hook(closeConnectionDueToProtocolError).Unhook,
		proto.Parser.Events.Error.Hook(closeConnectionDueToProtocolError).Unhook,
		proto.Parser.Events.Received[gossip.MessageTypeBlock].Hook(func(data []byte) {
			proto.Metrics.ReceivedBlocks.Inc()
			deps.ServerMetrics.Blocks.Inc()
			deps.MessageProcessor.Process(proto, gossip.MessageTypeBlock, data)
		}).Unhook,
		proto.Events.Sent[gossip.MessageTypeBlock].Hook(func() {
			proto.Metrics.SentPackets.Inc()
			proto.Metrics.SentBlocks.Inc()
			deps.ServerMetrics.SentBlocks.Inc()
		}).Unhook,
		proto.Parser.Events.Received[gossip.MessageTypeBlockRequest].Hook(func(data []byte) {
			proto.Metrics.ReceivedBlockRequests.Inc()
			deps.ServerMetrics.ReceivedBlockRequests.Inc()
			deps.MessageProcessor.Process(proto, gossip.MessageTypeBlockRequest, data)
		}).Unhook,
		proto.Events.Sent[gossip.MessageTypeBlockRequest].Hook(func() {
			proto.Metrics.SentPackets.Inc()
			proto.Metrics.SentBlockRequests.Inc()
			deps.ServerMetrics.SentBlockRequests.Inc()
		}).Unhook,
		proto.Parser.Events.Received[gossip.MessageTypeMilestoneRequest].Hook(func(data []byte) {
			proto.Metrics.ReceivedMilestoneRequests.Inc()
			deps.ServerMetrics.ReceivedMilestoneRequests.Inc()
			deps.MessageProcessor.Process(proto, gossip.MessageTypeMilestoneRequest, data)
		}).Unhook,
		proto.Events.Sent[gossip.MessageTypeMilestoneRequest].Hook(func() {
			proto.Metrics.SentPackets.Inc()
			proto.Metrics.SentMilestoneRequests.Inc()
			deps.ServerMetrics.SentMilestoneRequests.Inc()
		}).Unhook,
		proto.Parser.Events.Received[gossip.MessageTypeHeartbeat].Hook(func(data []byte) {
			proto.Metrics.ReceivedHeartbeats.Inc()
			deps.ServerMetrics.ReceivedHeartbeats.Inc()

			proto.LatestHeartbeat = gossip.ParseHeartbeat(data)
			proto.HeartbeatReceivedTime = time.Now()
			proto.Events.HeartbeatUpdated.Trigger(proto.LatestHeartbeat)
		}).Unhook,
		proto.Events.Sent[gossip.MessageTypeHeartbeat].Hook(func() {
			proto.Metrics.SentPackets.Inc()
			proto.Metrics.SentHeartbeats.Inc()
			deps.ServerMetrics.SentHeartbeats.Inc()
			proto.HeartbeatSentTime = time.Now()
		}).Unhook,
	))
}

func unhookGossipProtocolEvents(proto *gossip.Protocol) {
	gossipProtocolHooksMutex.Lock()
	defer gossipProtocolHooksMutex.Unlock()

	if unhook, exists := gossipProtocolHooks.Get(proto.PeerID); exists {
		unhook()
		gossipProtocolHooks.Delete(proto.PeerID)
	}
}

func unhookAllGossipProtocolEvents() {
	gossipProtocolHooksMutex.Lock()
	defer gossipProtocolHooksMutex.Unlock()

	gossipProtocolHooks.ForEach(func(id peer.ID, unhook func()) bool {
		unhook()
		gossipProtocolHooks.Delete(id)
		return true
	})
}
