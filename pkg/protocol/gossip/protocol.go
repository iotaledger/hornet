package gossip

import (
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/protocol"
)

const (
	// defines how far back a node's confirmed milestone index can be
	// but still considered synchronized.
	minCMISynchronizationThreshold = 2
)

// ProtocolEvents happening on a Protocol.
type ProtocolEvents struct {
	// Fired when the heartbeat message state on the peer has been updated.
	HeartbeatUpdated *events.Event
	// Fired when a message of the given type is sent.
	// This exists solely because protocol.Protocol in hive.go doesn't
	// emit events anymore for sent messages, as it is solely a parser.
	Sent []*events.Event
	// Fired when the protocol stream has been closed.
	Closed *events.Event
	// Fired when an error occurs on the protocol.
	Errors *events.Event
}

// NewProtocol creates a new gossip protocol instance associated to the given peer.
func NewProtocol(peerID peer.ID, stream network.Stream, sendQueueSize int, readTimeout, writeTimeout time.Duration, serverMetrics *metrics.ServerMetrics) *Protocol {
	defs := gossipMessageRegistry.Definitions()
	sentEvents := make([]*events.Event, len(defs))
	for i, def := range defs {
		if def == nil {
			continue
		}
		sentEvents[i] = events.NewEvent(events.VoidCaller)
	}

	return &Protocol{
		Parser: protocol.New(gossipMessageRegistry),
		PeerID: peerID,
		Events: ProtocolEvents{
			HeartbeatUpdated: events.NewEvent(HeartbeatCaller),
			// we need this because protocol.Protocol doesn't emit
			// events for sent messages anymore.
			Sent:   sentEvents,
			Closed: events.NewEvent(events.VoidCaller),
			Errors: events.NewEvent(events.ErrorCaller),
		},
		Stream:        stream,
		SendQueue:     make(chan []byte, sendQueueSize),
		readTimeout:   readTimeout,
		writeTimeout:  writeTimeout,
		ServerMetrics: serverMetrics,
	}
}

// Protocol represents an instance of the gossip protocol.
type Protocol struct {
	// Parser parses gossip messages and emits received events for them.
	Parser *protocol.Protocol
	// The ID of the peer to which this protocol is associated to.
	PeerID peer.ID
	// The underlying stream for this Protocol.
	Stream network.Stream
	// The events surrounding a Protocol.
	Events ProtocolEvents
	// The peer's latest heartbeat message.
	LatestHeartbeat *Heartbeat
	// Time the last heartbeat was received.
	HeartbeatReceivedTime time.Time
	// Time the last heartbeat was sent.
	HeartbeatSentTime time.Time
	// The send queue into which to enqueue messages to send.
	SendQueue chan []byte
	// The metrics around this protocol instance.
	Metrics      Metrics
	sendMu       sync.Mutex
	readTimeout  time.Duration
	writeTimeout time.Duration
	// The shared server metrics instance.
	ServerMetrics *metrics.ServerMetrics
}

// Enqueue enqueues the given gossip protocol message to be sent to the peer.
// If it can't because the send queue is over capacity, the message gets dropped.
func (p *Protocol) Enqueue(data []byte) {
	select {
	case p.SendQueue <- data:
	default:
		p.ServerMetrics.DroppedMessages.Inc()
		p.Metrics.DroppedPackets.Inc()
	}
}

// Read reads from the stream into the given buffer.
func (p *Protocol) Read(buf []byte) (int, error) {
	if err := p.Stream.SetReadDeadline(time.Now().Add(p.readTimeout)); err != nil {
		return 0, fmt.Errorf("unable to set read deadline: %w", err)
	}
	r, err := p.Stream.Read(buf)
	return r, err
}

// Send sends the given gossip message on the underlying Protocol.Stream.
func (p *Protocol) Send(message []byte) error {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()

	if err := p.Stream.SetWriteDeadline(time.Now().Add(p.writeTimeout)); err != nil {
		return fmt.Errorf("unable to set write deadline: %w", err)
	}

	// write message
	if _, err := p.Stream.Write(message); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// fire event handler for sent message
	p.Events.Sent[message[0]].Trigger()
	return nil
}

// SendMessage sends a storage.Message to the given peer.
func (p *Protocol) SendMessage(msgData []byte) {
	messageMsg, err := NewMessageMsg(msgData)
	if err != nil {
		return
	}
	p.Enqueue(messageMsg)
}

// SendHeartbeat sends a Heartbeat to the given peer.
func (p *Protocol) SendHeartbeat(solidMsIndex milestone.Index, pruningMsIndex milestone.Index, latestMsIndex milestone.Index, connectedNeighbors uint8, syncedNeighbors uint8) {
	heartbeatData, err := NewHeartbeatMsg(solidMsIndex, pruningMsIndex, latestMsIndex, connectedNeighbors, syncedNeighbors)
	if err != nil {
		return
	}
	p.Enqueue(heartbeatData)
}

// SendMessageRequest sends a storage.Message request message to the given peer.
func (p *Protocol) SendMessageRequest(requestedMessageID hornet.MessageID) {
	txReqData, err := NewMessageRequestMsg(requestedMessageID)
	if err != nil {
		return
	}
	p.Enqueue(txReqData)
}

// SendMilestoneRequest sends a storage.Milestone request to the given peer.
func (p *Protocol) SendMilestoneRequest(index milestone.Index) {
	milestoneRequestData, err := NewMilestoneRequestMsg(index)
	if err != nil {
		return
	}
	p.Enqueue(milestoneRequestData)
}

// SendLatestMilestoneRequest sends a storage.Milestone request which requests the latest known milestone from the given peer.
func (p *Protocol) SendLatestMilestoneRequest() {
	p.SendMilestoneRequest(LatestMilestoneRequestIndex)
}

// HasDataForMilestone tells whether the underlying peer given the latest heartbeat message, has the cone data for the given milestone.
// Returns false if no heartbeat message was received yet.
func (p *Protocol) HasDataForMilestone(index milestone.Index) bool {
	if p.LatestHeartbeat == nil {
		return false
	}
	return p.LatestHeartbeat.PrunedMilestoneIndex < index && p.LatestHeartbeat.SolidMilestoneIndex >= index
}

// CouldHaveDataForMilestone tells whether the underlying peer given the latest heartbeat message, could have parts of the cone data for the given milestone.
// Returns false if no heartbeat message was received yet.
func (p *Protocol) CouldHaveDataForMilestone(index milestone.Index) bool {
	if p.LatestHeartbeat == nil {
		return false
	}
	return p.LatestHeartbeat.PrunedMilestoneIndex < index && p.LatestHeartbeat.LatestMilestoneIndex >= index
}

// IsSynced tells whether the underlying peer is synced.
func (p *Protocol) IsSynced(cmi milestone.Index) bool {
	if p.LatestHeartbeat == nil {
		return false
	}

	latestIndex := p.LatestHeartbeat.LatestMilestoneIndex
	if latestIndex < cmi {
		latestIndex = cmi
	}

	if p.LatestHeartbeat.SolidMilestoneIndex < (latestIndex - minCMISynchronizationThreshold) {
		return false
	}

	return true
}

// Info returns
func (p *Protocol) Info() *Info {
	return &Info{
		Heartbeat: p.LatestHeartbeat,
		Metrics:   p.Metrics.Snapshot(),
	}
}

// Metrics defines a set of metrics regarding a gossip protocol instance.
type Metrics struct {
	// The number of received messages which are new.
	NewMessages atomic.Uint32
	// The number of received messages which are already known.
	KnownMessages atomic.Uint32
	// The number of received messages.
	ReceivedMessages atomic.Uint32
	// The number of received message requests.
	ReceivedMessageRequests atomic.Uint32
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint32
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint32
	// The number of sent packets.
	SentPackets atomic.Uint32
	// The number of sent messages.
	SentMessages atomic.Uint32
	// The number of sent message requests.
	SentMessageRequests atomic.Uint32
	// The number of sent milestone requests.
	SentMilestoneRequests atomic.Uint32
	// The number of sent heartbeats.
	SentHeartbeats atomic.Uint32
	// The number of dropped packets.
	DroppedPackets atomic.Uint32
}

// Snapshot returns MetricsSnapshot of the Metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		ReceivedMessages:     m.ReceivedMessages.Load(),
		NewMessages:          m.NewMessages.Load(),
		KnownMessages:        m.KnownMessages.Load(),
		ReceivedMessageReq:   m.ReceivedMessageRequests.Load(),
		ReceivedMilestoneReq: m.ReceivedMilestoneRequests.Load(),
		ReceivedHeartbeats:   m.ReceivedHeartbeats.Load(),
		SentMessages:         m.SentMessages.Load(),
		SentMessageReq:       m.SentMessageRequests.Load(),
		SentMilestoneReq:     m.SentMilestoneRequests.Load(),
		SentHeartbeats:       m.SentHeartbeats.Load(),
		DroppedPackets:       m.DroppedPackets.Load(),
	}
}

// MetricsSnapshot represents a snapshot of the gossip protocol metrics.
type MetricsSnapshot struct {
	NewMessages          uint32 `json:"newMessages"`
	KnownMessages        uint32 `json:"knownMessages"`
	ReceivedMessages     uint32 `json:"receivedMessages"`
	ReceivedMessageReq   uint32 `json:"receivedMessageRequests"`
	ReceivedMilestoneReq uint32 `json:"receivedMilestoneRequests"`
	ReceivedHeartbeats   uint32 `json:"receivedHeartbeats"`
	SentMessages         uint32 `json:"sentMessages"`
	SentMessageReq       uint32 `json:"sentMessageRequests"`
	SentMilestoneReq     uint32 `json:"sentMilestoneRequests"`
	SentHeartbeats       uint32 `json:"sentHeartbeats"`
	DroppedPackets       uint32 `json:"droppedPackets"`
}

// Info represents information about an ongoing gossip protocol.
type Info struct {
	Heartbeat *Heartbeat      `json:"heartbeat"`
	Metrics   MetricsSnapshot `json:"metrics"`
}
