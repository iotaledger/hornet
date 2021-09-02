package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/typeutils"
)

// ServiceEvents are events happening around a Service.
type ServiceEvents struct {
	// Fired when a protocol has been started.
	ProtocolStarted *events.Event
	// Fired when a protocol has ended.
	ProtocolTerminated *events.Event
	// Fired when an inbound stream gets canceled.
	InboundStreamCancelled *events.Event
	// Fired when an internal error happens.
	Error *events.Event
}

// ProtocolCaller gets called with a Protocol.
func ProtocolCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Protocol))(params[0].(*Protocol))
}

// StreamCaller gets called with a network.Stream.
func StreamCaller(handler interface{}, params ...interface{}) {
	handler.(func(network.Stream))(params[0].(network.Stream))
}

// StreamCancelCaller gets called with a network.Stream and its cancel reason.
func StreamCancelCaller(handler interface{}, params ...interface{}) {
	handler.(func(network.Stream, StreamCancelReason))(params[0].(network.Stream), params[1].(StreamCancelReason))
}

// StreamCancelReason is a reason for a gossip stream cancellation.
type StreamCancelReason string

const (
	// StreamCancelReasonDuplicated defines a stream cancellation because
	// it would lead to a duplicated ongoing stream.
	StreamCancelReasonDuplicated StreamCancelReason = "duplicated stream"
	// StreamCancelReasonInsufficientPeerRelation defines a stream cancellation because
	// the relation to the other peer is insufficient.
	StreamCancelReasonInsufficientPeerRelation StreamCancelReason = "insufficient peer relation"
	// StreamCancelReasonNoUnknownPeerSlotAvailable defines a stream cancellation
	// because no more unknown peers slot were available.
	StreamCancelReasonNoUnknownPeerSlotAvailable StreamCancelReason = "no unknown peer slot available"
	// StreamCancelReasonHostShutdown defines a stream cancellation
	// because the host is shutting down.
	StreamCancelReasonHostShutdown StreamCancelReason = "host shutdown"
)

const (
	defaultSendQueueSize        = 1000
	defaultStreamConnectTimeout = 4 * time.Second
)

// the default options applied to the Service.
var defaultServiceOptions = []ServiceOption{
	WithSendQueueSize(defaultSendQueueSize),
	WithStreamConnectTimeout(defaultStreamConnectTimeout),
	WithStreamReadTimeout(1 * time.Minute),
	WithStreamWriteTimeout(10 * time.Second),
	WithUnknownPeersLimit(0),
}

// ServiceOptions define options for a Service.
type ServiceOptions struct {
	// The size of the send queue buffer.
	SendQueueSize int
	// Timeout for connecting a stream.
	StreamConnectTimeout time.Duration
	// The read timeout for a stream.
	StreamReadTimeout time.Duration
	// The write timeout for a stream.
	StreamWriteTimeout time.Duration
	// The logger to use to log events.
	Logger *logger.Logger
	// The amount of unknown peers to allow to have a gossip stream with.
	UnknownPeersLimit int
}

// applies the given ServiceOption.
func (so *ServiceOptions) apply(opts ...ServiceOption) {
	for _, opt := range opts {
		opt(so)
	}
}

// WithSendQueueSize defines the size of send queues on ongoing gossip protocol streams.
func WithSendQueueSize(size int) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.SendQueueSize = size
	}
}

// WithStreamConnectTimeout defines the timeout for creating a gossip protocol stream.
func WithStreamConnectTimeout(dur time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.StreamConnectTimeout = dur
	}
}

// WithStreamReadTimeout defines the read timeout for reading from a stream.
func WithStreamReadTimeout(dur time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.StreamReadTimeout = dur
	}
}

// WithStreamWriteTimeout defines the write timeout for writing to a stream.
func WithStreamWriteTimeout(dur time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.StreamWriteTimeout = dur
	}
}

// WithLogger enables logging within the Service.
func WithLogger(logger *logger.Logger) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.Logger = logger
	}
}

// WithUnknownPeersLimit defines how many peers with an unknown relation
// are allowed to have an ongoing gossip protocol stream.
func WithUnknownPeersLimit(limit int) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.UnknownPeersLimit = limit
	}
}

// ServiceOption is a function setting a ServiceOptions option.
type ServiceOption func(opts *ServiceOptions)

// NewService creates a new Service.
func NewService(
	protocol protocol.ID, host host.Host,
	manager *p2p.Manager,
	serverMetrics *metrics.ServerMetrics, opts ...ServiceOption) *Service {
	srvOpts := &ServiceOptions{}
	srvOpts.apply(defaultServiceOptions...)
	srvOpts.apply(opts...)
	s := &Service{
		Events: ServiceEvents{
			ProtocolStarted:        events.NewEvent(ProtocolCaller),
			ProtocolTerminated:     events.NewEvent(ProtocolCaller),
			InboundStreamCancelled: events.NewEvent(StreamCancelCaller),
			Error:                  events.NewEvent(events.ErrorCaller),
		},
		host:                host,
		protocol:            protocol,
		streams:             make(map[peer.ID]*Protocol),
		manager:             manager,
		serverMetrics:       serverMetrics,
		opts:                srvOpts,
		stopped:             typeutils.NewAtomicBool(),
		unknownPeers:        map[peer.ID]struct{}{},
		inboundStreamChan:   make(chan network.Stream, 10),
		connectedChan:       make(chan *connectionmsg, 10),
		disconnectedChan:    make(chan *connectionmsg, 10),
		streamClosedChan:    make(chan *streamclosedmsg, 10),
		relationUpdatedChan: make(chan *relationupdatedmsg, 10),
		streamReqChan:       make(chan *streamreqmsg, 10),
		forEachChan:         make(chan *foreachmsg, 10),
	}
	if s.opts.Logger != nil {
		s.registerLoggerOnEvents()
	}
	return s
}

// Service handles ongoing gossip streams.
type Service struct {
	// Events happening around a Service.
	Events ServiceEvents
	// the libp2p host instance from which to work with.
	host     host.Host
	protocol protocol.ID
	// holds the set of protocols.
	streams map[peer.ID]*Protocol
	// the instance of the manager to work with.
	manager *p2p.Manager
	// the instance of the server metrics.
	serverMetrics *metrics.ServerMetrics
	// holds the service options.
	opts *ServiceOptions
	// tells whether the service was shut down.
	stopped *typeutils.AtomicBool
	// the amount of unknown peers with which a gossip stream is ongoing.
	unknownPeers map[peer.ID]struct{}
	// event loop channels
	inboundStreamChan   chan network.Stream
	connectedChan       chan *connectionmsg
	disconnectedChan    chan *connectionmsg
	streamClosedChan    chan *streamclosedmsg
	relationUpdatedChan chan *relationupdatedmsg
	streamReqChan       chan *streamreqmsg
	forEachChan         chan *foreachmsg
}

// Protocol returns the gossip.Protocol instance for the given peer or nil.
func (s *Service) Protocol(peerID peer.ID) *Protocol {
	if s.stopped.IsSet() {
		return nil
	}

	back := make(chan *Protocol)
	s.streamReqChan <- &streamreqmsg{peerID: peerID, back: back}
	return <-back
}

// ProtocolForEachFunc is used in Service.ForEach.
// Returning false indicates to stop looping.
// This function must not call any methods on Service.
type ProtocolForEachFunc func(proto *Protocol) bool

// ForEach calls the given ProtocolForEachFunc on each Protocol.
func (s *Service) ForEach(f ProtocolForEachFunc) {
	if s.stopped.IsSet() {
		return
	}

	back := make(chan struct{})
	s.forEachChan <- &foreachmsg{f: f, back: back}
	<-back
}

// SynchronizedCount returns the count of streams with peers
// which appear to be synchronized given their latest Heartbeat message.
func (s *Service) SynchronizedCount(latestMilestoneIndex milestone.Index) int {
	var count int
	s.ForEach(func(proto *Protocol) bool {
		if proto.IsSynced(latestMilestoneIndex) {
			count++
		}
		return true
	})
	return count
}

// Start starts the Service's event loop.
func (s *Service) Start(shutdownSignal <-chan struct{}) {
	s.host.SetStreamHandler(s.protocol, func(stream network.Stream) {
		if s.stopped.IsSet() {
			return
		}
		s.inboundStreamChan <- stream
	})
	s.manager.Events.Connected.Attach(events.NewClosure(func(peer *p2p.Peer, conn network.Conn) {
		if s.stopped.IsSet() {
			return
		}
		s.connectedChan <- &connectionmsg{peer: peer, conn: conn}
	}))
	s.manager.Events.Disconnected.Attach(events.NewClosure(func(peerOptErr *p2p.PeerOptError) {
		if s.stopped.IsSet() {
			return
		}
		s.disconnectedChan <- &connectionmsg{peer: peerOptErr.Peer, conn: nil}
	}))
	s.manager.Events.RelationUpdated.Attach(events.NewClosure(func(peer *p2p.Peer, oldRel p2p.PeerRelation) {
		if s.stopped.IsSet() {
			return
		}
		s.relationUpdatedChan <- &relationupdatedmsg{peer: peer, oldRelation: oldRel}
	}))
	// manage libp2p network events
	s.host.Network().Notify((*netNotifiee)(s))
	s.eventLoop(shutdownSignal)
	// de-register libp2p network events
	s.host.Network().StopNotify((*netNotifiee)(s))
}

// shutdown sets the stopped flag and drains all outstanding requests of the event loop.
func (s *Service) shutdown() {
	s.stopped.Set()

	// drain all outstanding requests of the event loop.
	// we do not care about correct handling of the channels, because we are shutting down anyway.
drainLoop:
	for {
		select {

		case <-s.inboundStreamChan:

		case <-s.connectedChan:

		case <-s.disconnectedChan:

		case <-s.streamClosedChan:

		case <-s.relationUpdatedChan:

		case streamReqMsg := <-s.streamReqChan:
			streamReqMsg.back <- nil

		case forEachMsg := <-s.forEachChan:
			forEachMsg.back <- struct{}{}

		default:
			break drainLoop
		}
	}
}

type connectionmsg struct {
	peer *p2p.Peer
	conn network.Conn
}

type streamreqmsg struct {
	peerID peer.ID
	back   chan *Protocol
}

type streamclosedmsg struct {
	peerID peer.ID
	stream network.Stream
}

type relationupdatedmsg struct {
	peer        *p2p.Peer
	oldRelation p2p.PeerRelation
}

type foreachmsg struct {
	f    ProtocolForEachFunc
	back chan struct{}
}

// runs the Service's event loop, handling inbound/outbound streams.
func (s *Service) eventLoop(shutdownSignal <-chan struct{}) {
	for {
		select {
		case <-shutdownSignal:
			s.shutdown()
			return

		case inboundStream := <-s.inboundStreamChan:
			if proto := s.handleInboundStream(inboundStream); proto != nil {
				s.Events.ProtocolStarted.Trigger(proto)
			}

		case connectedMsg := <-s.connectedChan:
			proto, err := s.handleConnected(connectedMsg.peer, connectedMsg.conn)
			if err != nil {
				s.Events.Error.Trigger(err)
				continue
			}

			if proto != nil {
				s.Events.ProtocolStarted.Trigger(proto)
			}

		case disconnectedMsg := <-s.disconnectedChan:
			s.closeStream(disconnectedMsg.peer.ID)

		case streamClosedMsg := <-s.streamClosedChan:
			s.closeStream(streamClosedMsg.peerID)

		case relationUpdatedMsg := <-s.relationUpdatedChan:
			proto, err := s.handleRelationUpdated(relationUpdatedMsg.peer, relationUpdatedMsg.oldRelation)
			if err != nil {
				s.Events.Error.Trigger(err)
				continue
			}

			if proto != nil {
				s.Events.ProtocolStarted.Trigger(proto)
			}

		case streamReqMsg := <-s.streamReqChan:
			streamReqMsg.back <- s.proto(streamReqMsg.peerID)

		case forEachMsg := <-s.forEachChan:
			s.forEach(forEachMsg.f)
			forEachMsg.back <- struct{}{}
		}
	}
}

// handles incoming streams and closes them if the given peer's relation should not allow any.
func (s *Service) handleInboundStream(stream network.Stream) *Protocol {
	remotePeerID := stream.Conn().RemotePeer()
	// close if there is already one
	if _, ongoing := s.streams[remotePeerID]; ongoing {
		s.Events.InboundStreamCancelled.Trigger(stream, StreamCancelReasonDuplicated)
		s.closeUnwantedStream(stream)
		return nil
	}

	// close if the relation to the peer is unknown and no slot is available
	var hasKnownRelation bool
	s.manager.Call(remotePeerID, func(peer *p2p.Peer) {
		if peer.Relation == p2p.PeerRelationUnknown {
			return
		}
		hasKnownRelation = true
	})

	var cancelReason StreamCancelReason
	switch {
	case !hasKnownRelation && s.opts.UnknownPeersLimit == 0:
		cancelReason = StreamCancelReasonInsufficientPeerRelation
	case !hasKnownRelation && len(s.unknownPeers) == s.opts.UnknownPeersLimit:
		cancelReason = StreamCancelReasonNoUnknownPeerSlotAvailable
	}

	if len(cancelReason) > 0 {
		s.Events.InboundStreamCancelled.Trigger(stream, cancelReason)
		s.closeUnwantedStream(stream)
		return nil
	}

	if !hasKnownRelation {
		s.unknownPeers[remotePeerID] = struct{}{}
	}

	return s.registerProtocol(remotePeerID, stream)
}

// closes the given unwanted stream by closing the underlying
// connection and the stream itself.
func (s *Service) closeUnwantedStream(stream network.Stream) {
	// using close and reset is the only way to make the remote's peer
	// "ClosedStream" notifiee handler fire: this is important, because
	// we want the remote peer to deregister the stream
	_ = stream.Conn().Close()
	_ = stream.Reset()
}

// handles the automatic creation of a protocol instance if the given peer
// was connected outbound and its peer relation allows it.
func (s *Service) handleConnected(peer *p2p.Peer, conn network.Conn) (*Protocol, error) {
	// don't create a new protocol if one is already ongoing
	if _, ongoing := s.streams[peer.ID]; ongoing {
		return nil, nil
	}

	// only initiate protocol if we connected outbound:
	// aka, handleInboundStream will be called for this connection
	if conn.Stat().Direction != network.DirOutbound {
		return nil, nil
	}

	if peer.Relation == p2p.PeerRelationUnknown {
		if len(s.unknownPeers) == s.opts.UnknownPeersLimit {
			return nil, nil
		}
		s.unknownPeers[peer.ID] = struct{}{}
	}

	stream, err := s.openStream(peer.ID)
	if err != nil {
		return nil, err
	}
	return s.registerProtocol(peer.ID, stream), nil
}

// opens up a stream to the given peer.
func (s *Service) openStream(peerID peer.ID) (network.Stream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.StreamConnectTimeout)
	defer cancel()

	stream, err := s.host.NewStream(ctx, peerID, s.protocol)
	if err != nil {
		return nil, fmt.Errorf("unable to create gossip stream to %s: %w", peerID, err)
	}
	// now some special sauce to trigger the remote peer's SetStreamHandler
	// https://github.com/libp2p/go-libp2p/issues/729
	_, _ = stream.Read(make([]byte, 0))
	return stream, nil
}

// registers a protocol instance for the given peer and stream.
func (s *Service) registerProtocol(peerID peer.ID, stream network.Stream) *Protocol {
	proto := NewProtocol(peerID, stream, s.opts.SendQueueSize, s.opts.StreamReadTimeout, s.opts.StreamWriteTimeout, s.serverMetrics)
	s.streams[peerID] = proto
	return proto
}

// deregisters ongoing gossip protocol streams and closes them for the given peer.
func (s *Service) deregisterProtocol(peerID peer.ID) (bool, error) {
	if _, ongoing := s.streams[peerID]; !ongoing {
		return false, nil
	}
	proto := s.streams[peerID]
	delete(s.streams, peerID)
	if err := proto.Stream.Reset(); err != nil {
		return true, fmt.Errorf("unable to cleanly reset stream to %s: %w", peerID, err)
	}
	return true, nil
}

// closes the stream for the given peer and fires the appropriate events.
func (s *Service) closeStream(peerID peer.ID) {
	proto := s.streams[peerID]
	delete(s.unknownPeers, peerID)
	reset, err := s.deregisterProtocol(peerID)
	if err != nil {
		s.Events.Error.Trigger(err)
		return
	}
	if reset {
		s.Events.ProtocolTerminated.Trigger(proto)
	}
}

// handles updates to the relation to a given peer: if the peer's relation
// is no longer unknown, a gossip protocol stream is started. likewise, if the
// relation is "downgraded" to unknown, the ongoing stream is closed if no more
// unknown peer slots are available.
func (s *Service) handleRelationUpdated(peer *p2p.Peer, oldRel p2p.PeerRelation) (*Protocol, error) {
	newRel := peer.Relation

	// close the stream if no more unknown peer slots are available
	if newRel == p2p.PeerRelationUnknown {
		if len(s.unknownPeers) == s.opts.UnknownPeersLimit {
			_, err := s.deregisterProtocol(peer.ID)
			return nil, err
		}
		s.unknownPeers[peer.ID] = struct{}{}
	}

	// clean up slot
	if oldRel == p2p.PeerRelationUnknown {
		delete(s.unknownPeers, peer.ID)
	}

	if _, ongoing := s.streams[peer.ID]; ongoing {
		return nil, nil
	}

	// here we might open a stream even if the connection is inbound:
	// the service should however take care of duplicated streams
	stream, err := s.openStream(peer.ID)
	if err != nil {
		return nil, err
	}

	return s.registerProtocol(peer.ID, stream), nil
}

// calls the given ProtocolForEachFunc on each protocol.
func (s *Service) forEach(f ProtocolForEachFunc) {
	for _, p := range s.streams {
		if s.stopped.IsSet() || !f(p) {
			break
		}
	}
}

// returns the protocol for the given peer or nil
func (s *Service) proto(peerID peer.ID) *Protocol {
	return s.streams[peerID]
}

// registers the logger on the events of the Service.
func (s *Service) registerLoggerOnEvents() {
	s.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *Protocol) {
		s.opts.Logger.Infof("started protocol with %s", proto.PeerID.ShortString())
	}))
	s.Events.ProtocolTerminated.Attach(events.NewClosure(func(proto *Protocol) {
		s.opts.Logger.Infof("terminated protocol with %s", proto.PeerID.ShortString())
	}))
	s.Events.InboundStreamCancelled.Attach(events.NewClosure(func(stream network.Stream, reason StreamCancelReason) {
		remotePeer := stream.Conn().RemotePeer().ShortString()
		s.opts.Logger.Infof("canceled inbound protocol stream from %s: %s", remotePeer, reason)
	}))
	s.Events.Error.Attach(events.NewClosure(func(err error) {
		s.opts.Logger.Error(err)
	}))
}

// lets Service implement network.Notifiee in order to automatically
// clean up ongoing reset streams
type netNotifiee Service

func (m *netNotifiee) Listen(net network.Network, multiaddr multiaddr.Multiaddr)      {}
func (m *netNotifiee) ListenClose(net network.Network, multiaddr multiaddr.Multiaddr) {}
func (m *netNotifiee) Connected(net network.Network, conn network.Conn)               {}
func (m *netNotifiee) Disconnected(net network.Network, conn network.Conn)            {}
func (m *netNotifiee) OpenedStream(net network.Network, stream network.Stream)        {}
func (m *netNotifiee) ClosedStream(net network.Network, stream network.Stream) {
	if stream.Protocol() != m.protocol {
		return
	}
	if m.stopped.IsSet() {
		return
	}
	m.streamClosedChan <- &streamclosedmsg{peerID: stream.Conn().RemotePeer(), stream: stream}
}
