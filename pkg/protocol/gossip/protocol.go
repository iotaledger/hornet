package gossip

import (
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/protocol"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	iotago "github.com/iotaledger/iota.go/v3"
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
		Events: &ProtocolEvents{
			HeartbeatUpdated: events.NewEvent(heartbeatCaller),
			// we need this because protocol.Protocol doesn't emit
			// events for sent messages anymore.
			Sent:   sentEvents,
			Errors: events.NewEvent(events.ErrorCaller),
		},
		Stream:         stream,
		terminatedChan: make(chan struct{}),
		SendQueue:      make(chan []byte, sendQueueSize),
		readTimeout:    readTimeout,
		writeTimeout:   writeTimeout,
		ServerMetrics:  serverMetrics,
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
	// terminatedChan is closed if the protocol was terminated.
	terminatedChan chan struct{}
	// The events surrounding a Protocol.
	Events *ProtocolEvents
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

// Terminated returns a channel that is closed if the protocol was terminated.
func (p *Protocol) Terminated() <-chan struct{} {
	return p.terminatedChan
}

// Enqueue enqueues the given gossip protocol message to be sent to the peer.
// If it can't because the send queue is over capacity, the message gets dropped.
func (p *Protocol) Enqueue(data []byte) {
	select {
	case p.SendQueue <- data:
	default:
		p.ServerMetrics.DroppedPackets.Inc()
		p.Metrics.DroppedPackets.Inc()
	}
}

// Read reads from the stream into the given buffer.
func (p *Protocol) Read(buf []byte) (int, error) {
	readMessage := func(buf []byte) (int, error) {
		if err := p.Stream.SetReadDeadline(time.Now().Add(p.readTimeout)); err != nil {
			return 0, fmt.Errorf("unable to set read deadline: %w", err)
		}

		return p.Stream.Read(buf)
	}

	r, err := readMessage(buf)
	if err != nil {
		p.Events.Errors.Trigger(err)
	}

	return r, err
}

// Send sends the given gossip message on the underlying Protocol.Stream.
func (p *Protocol) Send(message []byte) error {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()

	sendMessage := func(message []byte) error {
		if err := p.Stream.SetWriteDeadline(time.Now().Add(p.writeTimeout)); err != nil {
			return fmt.Errorf("unable to set write deadline: %w", err)
		}

		// write message
		if _, err := p.Stream.Write(message); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		return nil
	}

	if err := sendMessage(message); err != nil {
		p.Events.Errors.Trigger(err)

		return err
	}

	// fire event handler for sent message
	p.Events.Sent[message[0]].Trigger()

	return nil
}

// SendBlock sends a storage.Block to the given peer.
func (p *Protocol) SendBlock(blockData []byte) {
	blockMessage, err := newBlockMessage(blockData)
	if err != nil {
		return
	}
	p.Enqueue(blockMessage)
}

// SendHeartbeat sends a Heartbeat to the given peer.
func (p *Protocol) SendHeartbeat(solidMsIndex iotago.MilestoneIndex, pruningMsIndex iotago.MilestoneIndex, latestMsIndex iotago.MilestoneIndex, connectedPeers uint8, syncedPeers uint8) {
	heartbeatData, err := newHeartbeatMessage(solidMsIndex, pruningMsIndex, latestMsIndex, connectedPeers, syncedPeers)
	if err != nil {
		return
	}
	p.Enqueue(heartbeatData)
}

// SendBlockRequest sends a block request message to the given peer.
func (p *Protocol) SendBlockRequest(requestedBlockID iotago.BlockID) {
	blockRequestMessage, err := newBlockRequestMessage(requestedBlockID)
	if err != nil {
		return
	}
	p.Enqueue(blockRequestMessage)
}

// SendMilestoneRequest sends a milestone request to the given peer.
func (p *Protocol) SendMilestoneRequest(index iotago.MilestoneIndex) {
	milestoneRequestMessage, err := newMilestoneRequestMessage(index)
	if err != nil {
		return
	}
	p.Enqueue(milestoneRequestMessage)
}

// SendLatestMilestoneRequest sends a storage.Milestone request which requests the latest known milestone from the given peer.
func (p *Protocol) SendLatestMilestoneRequest() {
	p.SendMilestoneRequest(latestMilestoneRequestIndex)
}

// HasDataForMilestone tells whether the underlying peer given the latest heartbeat message, has the cone data for the given milestone.
// Returns false if no heartbeat message was received yet.
func (p *Protocol) HasDataForMilestone(index iotago.MilestoneIndex) bool {
	heartbeat := p.LatestHeartbeat
	if heartbeat == nil {
		return false
	}

	return heartbeat.PrunedMilestoneIndex < index && heartbeat.SolidMilestoneIndex >= index
}

// CouldHaveDataForMilestone tells whether the underlying peer given the latest heartbeat message, could have parts of the cone data for the given milestone.
// Returns false if no heartbeat message was received yet.
func (p *Protocol) CouldHaveDataForMilestone(index iotago.MilestoneIndex) bool {
	heartbeat := p.LatestHeartbeat
	if heartbeat == nil {
		return false
	}

	return heartbeat.PrunedMilestoneIndex < index && heartbeat.LatestMilestoneIndex >= index
}

// IsSynced tells whether the underlying peer is synced.
func (p *Protocol) IsSynced(cmi iotago.MilestoneIndex) bool {
	heartbeat := p.LatestHeartbeat
	if heartbeat == nil {
		return false
	}

	latestIndex := heartbeat.LatestMilestoneIndex
	if latestIndex < cmi {
		latestIndex = cmi
	}

	if heartbeat.SolidMilestoneIndex < (latestIndex - minCMISynchronizationThreshold) {
		return false
	}

	return true
}

// Info returns the info about the protocol.
func (p *Protocol) Info() *Info {
	return &Info{
		Heartbeat: p.LatestHeartbeat,
		Metrics:   p.Metrics.Snapshot(),
	}
}

// Metrics defines a set of metrics regarding a gossip protocol instance.
type Metrics struct {
	// The number of received blocks which are new.
	NewBlocks atomic.Uint32
	// The number of received blocks which are already known.
	KnownBlocks atomic.Uint32
	// The number of received blocks.
	ReceivedBlocks atomic.Uint32
	// The number of received block requests.
	ReceivedBlockRequests atomic.Uint32
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint32
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint32
	// The number of sent packets.
	SentPackets atomic.Uint32
	// The number of sent blocks.
	SentBlocks atomic.Uint32
	// The number of sent block requests.
	SentBlockRequests atomic.Uint32
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
		ReceivedBlocks:            m.ReceivedBlocks.Load(),
		NewBlocks:                 m.NewBlocks.Load(),
		KnownBlocks:               m.KnownBlocks.Load(),
		ReceivedBlockRequests:     m.ReceivedBlockRequests.Load(),
		ReceivedMilestoneRequests: m.ReceivedMilestoneRequests.Load(),
		ReceivedHeartbeats:        m.ReceivedHeartbeats.Load(),
		SentBlocks:                m.SentBlocks.Load(),
		SentBlockRequests:         m.SentBlockRequests.Load(),
		SentMilestoneRequests:     m.SentMilestoneRequests.Load(),
		SentHeartbeats:            m.SentHeartbeats.Load(),
		DroppedPackets:            m.DroppedPackets.Load(),
	}
}

// MetricsSnapshot represents a snapshot of the gossip protocol metrics.
type MetricsSnapshot struct {
	NewBlocks                 uint32 `json:"newBlocks"`
	KnownBlocks               uint32 `json:"knownBlocks"`
	ReceivedBlocks            uint32 `json:"receivedBlocks"`
	ReceivedBlockRequests     uint32 `json:"receivedBlockRequests"`
	ReceivedMilestoneRequests uint32 `json:"receivedMilestoneRequests"`
	ReceivedHeartbeats        uint32 `json:"receivedHeartbeats"`
	SentBlocks                uint32 `json:"sentBlocks"`
	SentBlockRequests         uint32 `json:"sentBlockRequests"`
	SentMilestoneRequests     uint32 `json:"sentMilestoneRequests"`
	SentHeartbeats            uint32 `json:"sentHeartbeats"`
	DroppedPackets            uint32 `json:"droppedPackets"`
}

// Info represents information about an ongoing gossip protocol.
type Info struct {
	Heartbeat *Heartbeat      `json:"heartbeat"`
	Metrics   MetricsSnapshot `json:"metrics"`
}
