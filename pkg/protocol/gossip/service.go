package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/utils"
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

var (
	ErrProtocolDoesNotExist = errors.New("stream/protocol does not exist")
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
	// The logger to use to logger events.
	logger *logger.Logger
	// The size of the send queue buffer.
	sendQueueSize int
	// Timeout for connecting a stream.
	streamConnectTimeout time.Duration
	// The read timeout for a stream.
	streamReadTimeout time.Duration
	// The write timeout for a stream.
	streamWriteTimeout time.Duration
	// The amount of unknown peers to allow to have a gossip stream with.
	unknownPeersLimit int
}

// applies the given ServiceOption.
func (so *ServiceOptions) apply(opts ...ServiceOption) {
	for _, opt := range opts {
		opt(so)
	}
}

// WithLogger enables logging within the Service.
func WithLogger(logger *logger.Logger) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.logger = logger
	}
}

// WithSendQueueSize defines the size of send queues on ongoing gossip protocol streams.
func WithSendQueueSize(size int) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.sendQueueSize = size
	}
}

// WithStreamConnectTimeout defines the timeout for creating a gossip protocol stream.
func WithStreamConnectTimeout(dur time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.streamConnectTimeout = dur
	}
}

// WithStreamReadTimeout defines the read timeout for reading from a stream.
func WithStreamReadTimeout(dur time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.streamReadTimeout = dur
	}
}

// WithStreamWriteTimeout defines the write timeout for writing to a stream.
func WithStreamWriteTimeout(dur time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.streamWriteTimeout = dur
	}
}

// WithUnknownPeersLimit defines how many peers with an unknown relation
// are allowed to have an ongoing gossip protocol stream.
func WithUnknownPeersLimit(limit int) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.unknownPeersLimit = limit
	}
}

// ServiceOption is a function setting a ServiceOptions option.
type ServiceOption func(opts *ServiceOptions)

// Service handles ongoing gossip streams.
type Service struct {
	// the logger used to log events.
	*utils.WrappedLogger

	// Events happening around a Service.
	Events *ServiceEvents
	// the libp2p host instance from which to work with.
	host     host.Host
	protocol protocol.ID
	// holds the set of protocols.
	streams map[peer.ID]*Protocol
	// the instance of the peeringManager to work with.
	peeringManager *p2p.Manager
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
	closeStreamChan     chan *closestreammsg
	disconnectedChan    chan *connectionmsg
	streamClosedChan    chan *streamclosedmsg
	relationUpdatedChan chan *relationupdatedmsg
	streamReqChan       chan *streamreqmsg
	forEachChan         chan *foreachmsg

	// closures
	// peering manager
	onPeeringManagerConnected       *events.Closure
	onPeeringManagerDisconnected    *events.Closure
	onPeeringManagerRelationUpdated *events.Closure

	// logger
	onGossipServiceProtocolStarted        *events.Closure
	onGossipServiceProtocolTerminated     *events.Closure
	onGossipServiceInboundStreamCancelled *events.Closure
	onGossipServiceError                  *events.Closure
}

// NewService creates a new Service.
func NewService(
	protocol protocol.ID, host host.Host,
	peeringManager *p2p.Manager,
	serverMetrics *metrics.ServerMetrics,
	opts ...ServiceOption) *Service {

	srvOpts := &ServiceOptions{}
	srvOpts.apply(defaultServiceOptions...)
	srvOpts.apply(opts...)

	gossipService := &Service{
		Events: &ServiceEvents{
			ProtocolStarted:        events.NewEvent(ProtocolCaller),
			ProtocolTerminated:     events.NewEvent(ProtocolCaller),
			InboundStreamCancelled: events.NewEvent(StreamCancelCaller),
			Error:                  events.NewEvent(events.ErrorCaller),
		},
		host:                host,
		protocol:            protocol,
		streams:             make(map[peer.ID]*Protocol),
		peeringManager:      peeringManager,
		serverMetrics:       serverMetrics,
		opts:                srvOpts,
		stopped:             typeutils.NewAtomicBool(),
		unknownPeers:        map[peer.ID]struct{}{},
		inboundStreamChan:   make(chan network.Stream, 10),
		connectedChan:       make(chan *connectionmsg, 10),
		closeStreamChan:     make(chan *closestreammsg, 10),
		disconnectedChan:    make(chan *connectionmsg, 10),
		streamClosedChan:    make(chan *streamclosedmsg, 10),
		relationUpdatedChan: make(chan *relationupdatedmsg, 10),
		streamReqChan:       make(chan *streamreqmsg, 10),
		forEachChan:         make(chan *foreachmsg, 10),
	}
	gossipService.WrappedLogger = utils.NewWrappedLogger(gossipService.opts.logger)
	gossipService.configureEvents()
	return gossipService
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

// CloseStream closes an ongoing stream with a peer.
func (s *Service) CloseStream(peerID peer.ID) error {
	if s.stopped.IsSet() {
		return nil
	}

	back := make(chan error)
	s.closeStreamChan <- &closestreammsg{peerID: peerID, back: back}
	return <-back
}

// Start starts the Service's event loop.
func (s *Service) Start(ctx context.Context) {

	s.attachEvents()

	// libp2p stream handler
	s.host.SetStreamHandler(s.protocol, func(stream network.Stream) {
		if s.stopped.IsSet() {
			return
		}
		s.inboundStreamChan <- stream
	})

	// manage libp2p network events
	s.host.Network().Notify((*netNotifiee)(s))

	s.eventLoop(ctx)

	// libp2p stream handler
	s.host.RemoveStreamHandler(s.protocol)

	// de-register libp2p network events
	s.host.Network().StopNotify((*netNotifiee)(s))

	s.detachEvents()
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

		case <-s.closeStreamChan:
		case disconnectMsg := <-s.closeStreamChan:
			disconnectMsg.back <- nil

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

type closestreammsg struct {
	peerID peer.ID
	back   chan error
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
func (s *Service) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.shutdown()
			return

		case inboundStream := <-s.inboundStreamChan:
			s.handleInboundStream(inboundStream)

		case connectedMsg := <-s.connectedChan:
			s.handleConnected(connectedMsg.peer, connectedMsg.conn)

		case disconnectMsg := <-s.closeStreamChan:
			if err := s.deregisterProtocol(disconnectMsg.peerID); err != nil && !errors.Is(err, ErrProtocolDoesNotExist) {
				disconnectMsg.back <- err
			}
			disconnectMsg.back <- nil

		case disconnectedMsg := <-s.disconnectedChan:
			if err := s.deregisterProtocol(disconnectedMsg.peer.ID); err != nil && !errors.Is(err, ErrProtocolDoesNotExist) {
				s.Events.Error.Trigger(err)
			}

		case streamClosedMsg := <-s.streamClosedChan:
			if err := s.deregisterProtocol(streamClosedMsg.peerID); err != nil && !errors.Is(err, ErrProtocolDoesNotExist) {
				s.Events.Error.Trigger(err)
			}

		case relationUpdatedMsg := <-s.relationUpdatedChan:
			s.handleRelationUpdated(relationUpdatedMsg.peer, relationUpdatedMsg.oldRelation)

		case streamReqMsg := <-s.streamReqChan:
			streamReqMsg.back <- s.proto(streamReqMsg.peerID)

		case forEachMsg := <-s.forEachChan:
			s.forEach(forEachMsg.f)
			forEachMsg.back <- struct{}{}
		}
	}
}

// handles incoming streams and closes them if the given peer's relation should not allow any.
func (s *Service) handleInboundStream(stream network.Stream) {
	remotePeerID := stream.Conn().RemotePeer()

	// close if there is already one
	if _, ongoing := s.streams[remotePeerID]; ongoing {
		s.Events.InboundStreamCancelled.Trigger(stream, StreamCancelReasonDuplicated)
		s.closeUnwantedStream(stream)
		return
	}

	// close if the relation to the peer is unknown and no slot is available
	hasUnknownRelation := true
	s.peeringManager.Call(remotePeerID, func(peer *p2p.Peer) {
		switch peer.Relation {
		case p2p.PeerRelationAutopeered:
			hasUnknownRelation = false
		case p2p.PeerRelationKnown:
			hasUnknownRelation = false
		}
	})

	var cancelReason StreamCancelReason
	if hasUnknownRelation {
		switch {
		case s.opts.unknownPeersLimit == 0:
			cancelReason = StreamCancelReasonInsufficientPeerRelation
		case len(s.unknownPeers) >= s.opts.unknownPeersLimit:
			cancelReason = StreamCancelReasonNoUnknownPeerSlotAvailable
		}
	}

	if len(cancelReason) > 0 {
		s.Events.InboundStreamCancelled.Trigger(stream, cancelReason)
		s.closeUnwantedStream(stream)
		return
	}

	if hasUnknownRelation {
		s.unknownPeers[remotePeerID] = struct{}{}
	}

	s.registerProtocol(remotePeerID, stream)
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
func (s *Service) handleConnected(peer *p2p.Peer, conn network.Conn) {

	connect := func() error {
		// don't create a new protocol if one is already ongoing
		if _, ongoing := s.streams[peer.ID]; ongoing {
			return nil
		}

		// only initiate protocol if we connected outbound:
		// aka, handleInboundStream will be called for this connection
		if conn.Stat().Direction != network.DirOutbound {
			return nil
		}

		if peer.Relation == p2p.PeerRelationUnknown {
			if len(s.unknownPeers) >= s.opts.unknownPeersLimit {
				return nil
			}
			s.unknownPeers[peer.ID] = struct{}{}
		}

		stream, err := s.openStream(peer.ID)
		if err != nil {
			return err
		}

		s.registerProtocol(peer.ID, stream)
		return nil
	}

	if err := connect(); err != nil {
		s.Events.Error.Trigger(err)
	}
}

// opens up a stream to the given peer.
func (s *Service) openStream(peerID peer.ID) (network.Stream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.streamConnectTimeout)
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
func (s *Service) registerProtocol(peerID peer.ID, stream network.Stream) {
	// don't create a new protocol if one is already ongoing
	if _, ongoing := s.streams[peerID]; ongoing {
		return
	}

	proto := NewProtocol(peerID, stream, s.opts.sendQueueSize, s.opts.streamReadTimeout, s.opts.streamWriteTimeout, s.serverMetrics)
	s.streams[peerID] = proto
	s.Events.ProtocolStarted.Trigger(proto)
}

// deregisters ongoing gossip protocol streams and closes them for the given peer.
func (s *Service) deregisterProtocol(peerID peer.ID) error {
	proto, ongoing := s.streams[peerID]
	if !ongoing {
		return fmt.Errorf("unable to deregister protocol %s: %w", peerID, ErrProtocolDoesNotExist)
	}

	defer func() {
		delete(s.streams, peerID)
		delete(s.unknownPeers, peerID)
		close(proto.terminatedChan)
		s.Events.ProtocolTerminated.Trigger(proto)
	}()

	if err := proto.Stream.Reset(); err != nil {
		return fmt.Errorf("unable to cleanly reset stream to %s: %w", peerID, err)
	}

	return nil
}

// handles updates to the relation to a given peer: if the peer's relation
// is no longer unknown, a gossip protocol stream is started. likewise, if the
// relation is "downgraded" to unknown, the ongoing stream is closed if no more
// unknown peer slots are available.
func (s *Service) handleRelationUpdated(peer *p2p.Peer, oldRel p2p.PeerRelation) {
	newRel := peer.Relation

	updateRelation := func() error {
		if newRel == p2p.PeerRelationUnknown {
			if len(s.unknownPeers) >= s.opts.unknownPeersLimit {
				// close the stream if no more unknown peer slots are available
				err := s.deregisterProtocol(peer.ID)
				return err
			}
			s.unknownPeers[peer.ID] = struct{}{}
		}

		// clean up slot
		if oldRel == p2p.PeerRelationUnknown {
			delete(s.unknownPeers, peer.ID)
		}

		// don't create a new protocol if one is already ongoing
		if _, ongoing := s.streams[peer.ID]; ongoing {
			return nil
		}

		// here we might open a stream even if the connection is inbound:
		// the service should however take care of duplicated streams
		stream, err := s.openStream(peer.ID)
		if err != nil {
			return err
		}

		s.registerProtocol(peer.ID, stream)

		return nil
	}

	if err := updateRelation(); err != nil {
		s.Events.Error.Trigger(err)
	}
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

func (s *Service) configureEvents() {

	// peering manager
	s.onPeeringManagerConnected = events.NewClosure(func(peer *p2p.Peer, conn network.Conn) {
		if s.stopped.IsSet() {
			return
		}
		s.connectedChan <- &connectionmsg{peer: peer, conn: conn}
	})

	s.onPeeringManagerDisconnected = events.NewClosure(func(peerOptErr *p2p.PeerOptError) {
		if s.stopped.IsSet() {
			return
		}
		s.disconnectedChan <- &connectionmsg{peer: peerOptErr.Peer, conn: nil}
	})

	s.onPeeringManagerRelationUpdated = events.NewClosure(func(peer *p2p.Peer, oldRel p2p.PeerRelation) {
		if s.stopped.IsSet() {
			return
		}
		s.relationUpdatedChan <- &relationupdatedmsg{peer: peer, oldRelation: oldRel}
	})

	// logger
	s.onGossipServiceProtocolStarted = events.NewClosure(func(proto *Protocol) {
		s.LogInfof("started protocol with %s", proto.PeerID.ShortString())
	})

	s.onGossipServiceProtocolTerminated = events.NewClosure(func(proto *Protocol) {
		s.LogInfof("terminated protocol with %s", proto.PeerID.ShortString())
	})

	s.onGossipServiceInboundStreamCancelled = events.NewClosure(func(stream network.Stream, reason StreamCancelReason) {
		remotePeer := stream.Conn().RemotePeer().ShortString()
		s.LogInfof("canceled inbound protocol stream from %s: %s", remotePeer, reason)
	})

	s.onGossipServiceError = events.NewClosure(func(err error) {
		s.LogWarn(err)
	})
}

func (s *Service) attachEvents() {
	// peering manager
	s.peeringManager.Events.Connected.Attach(s.onPeeringManagerConnected)
	s.peeringManager.Events.Disconnected.Attach(s.onPeeringManagerDisconnected)
	s.peeringManager.Events.RelationUpdated.Attach(s.onPeeringManagerRelationUpdated)

	// logger
	s.Events.ProtocolStarted.Attach(s.onGossipServiceProtocolStarted)
	s.Events.ProtocolTerminated.Attach(s.onGossipServiceProtocolTerminated)
	s.Events.InboundStreamCancelled.Attach(s.onGossipServiceInboundStreamCancelled)
	s.Events.Error.Attach(s.onGossipServiceError)
}

func (s *Service) detachEvents() {
	// peering manager
	s.peeringManager.Events.Connected.Detach(s.onPeeringManagerConnected)
	s.peeringManager.Events.Disconnected.Detach(s.onPeeringManagerDisconnected)
	s.peeringManager.Events.RelationUpdated.Detach(s.onPeeringManagerRelationUpdated)

	// logger
	s.Events.ProtocolStarted.Detach(s.onGossipServiceProtocolStarted)
	s.Events.ProtocolTerminated.Detach(s.onGossipServiceProtocolTerminated)
	s.Events.InboundStreamCancelled.Detach(s.onGossipServiceInboundStreamCancelled)
	s.Events.Error.Detach(s.onGossipServiceError)
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
