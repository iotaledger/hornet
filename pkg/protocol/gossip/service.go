package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
)

// ServiceEvents are events happening around a Service.
type ServiceEvents struct {
	// Fired when a protocol has been started.
	ProtocolStarted *events.Event
	// Fired when a protocol has ended.
	ProtocolTerminated *events.Event
	// Fired when an inbound stream gets cancelled.
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

const (
	defaultSendQueueSize        = 1000
	defaultStreamConnectTimeout = 4 * time.Second
)

// the default options applied to the Service.
var defaultServiceOptions = []ServiceOption{
	WithSendQueueSize(defaultSendQueueSize),
	WithStreamConnectTimeout(defaultStreamConnectTimeout),
}

// ServiceOptions define options for a Service.
type ServiceOptions struct {
	SendQueueSize        int
	StreamConnectTimeout time.Duration
	Logger               *logger.Logger
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

// WithLogger enables logging within the Service.
func WithLogger(logger *logger.Logger) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.Logger = logger
	}
}

// ServiceOption is a function setting a ServiceOptions option.
type ServiceOption func(opts *ServiceOptions)

// NewService creates a new Service.
func NewService(protocol protocol.ID, host host.Host, manager *p2p.Manager, opts ...ServiceOption) *Service {
	srvOpts := &ServiceOptions{}
	srvOpts.apply(defaultServiceOptions...)
	srvOpts.apply(opts...)
	s := &Service{
		Events: ServiceEvents{
			ProtocolStarted:        events.NewEvent(ProtocolCaller),
			ProtocolTerminated:     events.NewEvent(ProtocolCaller),
			InboundStreamCancelled: events.NewEvent(StreamCaller),
			Error:                  events.NewEvent(events.ErrorCaller),
		},
		host:                host,
		protocol:            protocol,
		streams:             make(map[peer.ID]*Protocol),
		manager:             manager,
		opts:                srvOpts,
		inboundStreamChan:   make(chan network.Stream),
		connectedChan:       make(chan *connectionmsg),
		disconnectedChan:    make(chan *connectionmsg),
		streamClosedChan:    make(chan *streamclosedmsg),
		relationUpdatedChan: make(chan *relationupdatedmsg),
		streamReqChan:       make(chan *streamreqmsg),
		forEachChan:         make(chan *foreachmsg),
	}
	if s.opts.Logger != nil {
		s.registerLoggerOnEvents()
	}
	return s
}

// Service handles ongoing gossip streams.
type Service struct {
	// Events happening arounda Service.
	Events   ServiceEvents
	host     host.Host
	protocol protocol.ID
	streams  map[peer.ID]*Protocol
	manager  *p2p.Manager
	opts     *ServiceOptions
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
func (s *Service) Protocol(id peer.ID) *Protocol {
	back := make(chan *Protocol)
	s.streamReqChan <- &streamreqmsg{p: id, back: back}
	return <-back
}

// ProtocolForEachFunc is used in Service.ForEach.
// Returning false indicates to stop looping.
// This function must not call any methods on Service.
type ProtocolForEachFunc func(proto *Protocol) bool

// ForEach calls the given PeerForEachFunc on each Peer.
// Optionally only loops over the peers with the given filter relation.
func (s *Service) ForEach(f ProtocolForEachFunc) {
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
		s.inboundStreamChan <- stream
	})
	s.manager.Events.Connected.Attach(events.NewClosure(func(p *p2p.Peer, conn network.Conn) {
		s.connectedChan <- &connectionmsg{p: p, conn: conn}
	}))
	s.manager.Events.Disconnected.Attach(events.NewClosure(func(p *p2p.Peer) {
		s.disconnectedChan <- &connectionmsg{p: p, conn: nil}
	}))
	s.manager.Events.RelationUpdated.Attach(events.NewClosure(func(p *p2p.Peer, oldRel p2p.PeerRelation) {
		s.relationUpdatedChan <- &relationupdatedmsg{peer: p, oldRelation: oldRel}
	}))
	// manage libp2p network events
	s.host.Network().Notify((*netNotifiee)(s))
	s.eventLoop(shutdownSignal)
	// de-register libp2p network events
	s.host.Network().StopNotify((*netNotifiee)(s))
}

type connectionmsg struct {
	p    *p2p.Peer
	conn network.Conn
}

type streamreqmsg struct {
	p    peer.ID
	back chan *Protocol
}

type streamclosedmsg struct {
	p      peer.ID
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
			return

		case inboundStream := <-s.inboundStreamChan:
			if proto := s.handleInboundStream(inboundStream); proto != nil {
				s.Events.ProtocolStarted.Trigger(proto)
			}

		case connectedMsg := <-s.connectedChan:
			proto, err := s.handleConnected(connectedMsg.p, connectedMsg.conn)
			if err != nil {
				s.Events.Error.Trigger(err)
				continue
			}

			if proto != nil {
				s.Events.ProtocolStarted.Trigger(proto)
			}

		case disconnectedMsg := <-s.disconnectedChan:
			s.closeStream(disconnectedMsg.p.ID)

		case streamClosedMsg := <-s.streamClosedChan:
			s.closeStream(streamClosedMsg.p)

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
			streamReqMsg.back <- s.proto(streamReqMsg.p)

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
		s.closeUnwantedStream(stream)
		return nil
	}

	// close if the relation to the peer is unknown
	var ok bool
	s.manager.Call(remotePeerID, func(p *p2p.Peer) {
		if p.Relation == p2p.PeerRelationUnknown {
			return
		}
		ok = true
	})

	if !ok {
		s.Events.InboundStreamCancelled.Trigger(stream)
		s.closeUnwantedStream(stream)
		return nil
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
func (s *Service) handleConnected(p *p2p.Peer, conn network.Conn) (*Protocol, error) {
	// close if there is already one
	if _, ongoing := s.streams[p.ID]; ongoing {
		return nil, nil
	}

	// only initiate protocol if we connected outbound:
	// aka, handleInboundStream will be called for this connection
	if conn.Stat().Direction != network.DirOutbound {
		return nil, nil
	}

	stream, err := s.openStream(p.ID)
	if err != nil {
		return nil, err
	}
	return s.registerProtocol(p.ID, stream), nil
}

// opens up a stream to the given peer.
func (s *Service) openStream(id peer.ID) (network.Stream, error) {
	ctx, _ := context.WithTimeout(context.Background(), s.opts.StreamConnectTimeout)
	stream, err := s.host.NewStream(ctx, id, s.protocol)
	if err != nil {
		return nil, fmt.Errorf("unable to create gossip stream to %s: %w", id, err)
	}
	// now some special sauce to trigger the remote peer's SetStreamHandler
	// ¯\_(ツ)_/¯
	// https://github.com/libp2p/go-libp2p/issues/729
	_, _ = stream.Read(make([]byte, 0))
	return stream, nil
}

// registers a protocol instance for the given peer and stream.
func (s *Service) registerProtocol(id peer.ID, stream network.Stream) *Protocol {
	proto := NewProtocol(id, stream, s.opts.SendQueueSize)
	s.streams[id] = proto
	return proto
}

// deregisters ongoing gossip protocol streams and closes them for the given peer.
func (s *Service) deregisterProtocol(id peer.ID) (bool, error) {
	if _, ongoing := s.streams[id]; !ongoing {
		return false, nil
	}
	proto := s.streams[id]
	delete(s.streams, id)
	if err := proto.Stream.Reset(); err != nil {
		return true, fmt.Errorf("unable to cleanly reset stream to %s: %w", id, err)
	}
	return true, nil
}

// closes the stream for the given peer and fires the appropriate events.
func (s *Service) closeStream(id peer.ID) {
	proto := s.streams[id]
	reset, err := s.deregisterProtocol(id)
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
// relation is "downgraded" to unknown, the ongoing stream is closed.
func (s *Service) handleRelationUpdated(p *p2p.Peer, oldRel p2p.PeerRelation) (*Protocol, error) {
	newRel := p.Relation

	// close the stream
	if newRel == p2p.PeerRelationUnknown {
		_, err := s.deregisterProtocol(p.ID)
		return nil, err
	}

	if _, ongoing := s.streams[p.ID]; ongoing {
		return nil, nil
	}

	// here we might open a stream even if the connection is inbound:
	// the service should however take care of duplicated streams
	stream, err := s.openStream(p.ID)
	if err != nil {
		return nil, err
	}

	return s.registerProtocol(p.ID, stream), nil
}

// calls the given ProtocolForEachFunc on each protocol.
func (s *Service) forEach(f ProtocolForEachFunc) {
	for _, p := range s.streams {
		if !f(p) {
			break
		}
	}
}

// returns the protocol for the given peer or nil
func (s *Service) proto(id peer.ID) *Protocol {
	return s.streams[id]
}

// registers the logger on the events of the Service.
func (s *Service) registerLoggerOnEvents() {
	s.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *Protocol) {
		s.opts.Logger.Infof("started protocol with %s", proto.PeerID.ShortString())
	}))
	s.Events.ProtocolTerminated.Attach(events.NewClosure(func(proto *Protocol) {
		s.opts.Logger.Infof("terminated protocol with %s", proto.PeerID.ShortString())
	}))
	s.Events.InboundStreamCancelled.Attach(events.NewClosure(func(stream network.Stream) {
		remotePeer := stream.Conn().RemotePeer().ShortString()
		s.opts.Logger.Infof("cancelled inbound protocol stream from %s", remotePeer)
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
	m.streamClosedChan <- &streamclosedmsg{p: stream.Conn().RemotePeer(), stream: stream}
}
