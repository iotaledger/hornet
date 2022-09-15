package p2p

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/typeutils"
)

const (
	// PeerConnectivityProtectionTag is the tag used by Manager to
	// protect known peer connectivity from getting trimmed via the
	// connmgr.ConnManager.
	PeerConnectivityProtectionTag = "peering-manager"

	connTimeout = 5 * time.Second
)

var (
	// ErrCantConnectToItself gets returned if the manager is supposed to create a connection
	// to itself (host wise).
	ErrCantConnectToItself = errors.New("the host can't connect to itself")
	// ErrPeerInManagerAlready gets returned if a peer is tried to be added to the manager which is already added.
	ErrPeerInManagerAlready = errors.New("peer is already in manager")
	// ErrCantAllowItself gets returned if the manager is supposed to allow a connection
	// to itself (host wise).
	ErrCantAllowItself = errors.New("the host can't allow itself")
	// ErrPeerInManagerAlreadyAllowed gets returned if a peer is tried to be allowed in the manager which is already allowed.
	ErrPeerInManagerAlreadyAllowed = errors.New("peer is already allowed in manager")
	// ErrManagerShutdown gets returned if the manager is shutting down.
	ErrManagerShutdown = errors.New("manager is shutting down")
)

// PeerRelation defines the type of relation to a remote peer.
type PeerRelation string

const (
	// PeerRelationKnown is a relation to a peer which most
	// likely stems from knowing the operator of said peer.
	// Connections to such peers are subject to automatic reconnections.
	PeerRelationKnown PeerRelation = "known"
	// PeerRelationUnknown is a relation to an unknown peer.
	// Connections to such peers do not have to be retained.
	PeerRelationUnknown PeerRelation = "unknown"
	// PeerRelationAutopeered is a relation to an autopeered peer.
	// Connections to such peers do not have to be retained.
	PeerRelationAutopeered PeerRelation = "autopeered"
)

// ManagerState represents the state in which the Manager is in.
type ManagerState string

const (
	// ManagerStateStarted means that the Manager has been started.
	ManagerStateStarted ManagerState = "started"
	// ManagerStateStopping means that the Manager is halting its operation.
	ManagerStateStopping ManagerState = "stopping"
	// ManagerStateStopped means tha the Manager has halted its operation.
	ManagerStateStopped ManagerState = "stopped"
)

// ManagerEvents are events happening around a Manager.
// No methods on Manager must be called from within the event handlers.
type ManagerEvents struct {
	// Fired when the Manager is instructed to establish a connection to a peer.
	Connect *events.Event
	// Fired when the Manager is instructed to disconnect a peer.
	Disconnect *events.Event
	// Fired when a peer got allowed.
	Allowed *events.Event
	// Fired when a peer got disallowed.
	Disallowed *events.Event
	// Fired when a peer got connected.
	Connected *events.Event
	// Fired when a peer got disconnected.
	Disconnected *events.Event
	// Fired when a reconnect is scheduled.
	ScheduledReconnect *events.Event
	// Fired when the Manager tries to reconnect to a peer.
	Reconnecting *events.Event
	// Fired when a peer has been reconnected.
	Reconnected *events.Event
	// Fired when the relation to a peer has been updated.
	RelationUpdated *events.Event
	// Fired when the Manager's state changes.
	StateChange *events.Event
	// Fired when internal error happens.
	Error *events.Event
}

// PeerCaller gets called with a Peer.
func PeerCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(*Peer))(params[0].(*Peer))
}

// PeerIDCaller gets called with a peer.ID.
func PeerIDCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(peer.ID))(params[0].(peer.ID))
}

// PeerOptError holds a Peer and optionally an error.
type PeerOptError struct {
	Peer  *Peer
	Error error
}

// PeerOptErrorCaller gets called with a Peer and an error.
func PeerOptErrorCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(*PeerOptError))(params[0].(*PeerOptError))
}

// ManagerStateCaller gets called with a ManagerState.
func ManagerStateCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(ManagerState))(params[0].(ManagerState))
}

// PeerDurationCaller gets called with a Peer and a time.Duration.
func PeerDurationCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(*Peer, time.Duration))(params[0].(*Peer), params[1].(time.Duration))
}

// PeerConnCaller gets called with a Peer and its associated network.Conn.
func PeerConnCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(*Peer, network.Conn))(params[0].(*Peer), params[1].(network.Conn))
}

// PeerRelationCaller gets called with a Peer and its old PeerRelation.
func PeerRelationCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(p *Peer, old PeerRelation))(params[0].(*Peer), params[1].(PeerRelation))
}

// the default options applied to the Manager.
var defaultManagerOptions = []ManagerOption{
	WithManagerReconnectInterval(30*time.Second, 1*time.Second),
}

// ManagerOptions define options for a Manager.
type ManagerOptions struct {
	// The logger to use to logger events.
	logger *logger.Logger
	// The static reconnect interval.
	reconnectInterval time.Duration
	// The randomized jitter applied to the reconnect interval.
	reconnectIntervalJitter time.Duration
}

// ManagerOption is a function setting a ManagerOptions option.
type ManagerOption func(opts *ManagerOptions)

// WithManagerLogger enables logging within the Manager.
func WithManagerLogger(logger *logger.Logger) ManagerOption {
	return func(opts *ManagerOptions) {
		opts.logger = logger
	}
}

// WithManagerReconnectInterval defines the re-connect interval for peers
// to which the Manager wants to keep a connection open to.
func WithManagerReconnectInterval(interval time.Duration, jitter time.Duration) ManagerOption {
	return func(opts *ManagerOptions) {
		opts.reconnectInterval = interval
		opts.reconnectIntervalJitter = jitter
	}
}

// applies the given ManagerOption.
func (mo *ManagerOptions) apply(opts ...ManagerOption) {
	for _, opt := range opts {
		opt(mo)
	}
}

// computes the next reconnect delay.
func (mo *ManagerOptions) reconnectDelay() time.Duration {
	recInter := mo.reconnectInterval
	jitter := mo.reconnectIntervalJitter
	//nolint:gosec // we don't care about weak random numbers here
	delayJitter := rand.Int63n(int64(jitter))

	return recInter + time.Duration(delayJitter)
}

// NewManager creates a new Manager.
func NewManager(host host.Host, opts ...ManagerOption) *Manager {
	mngOpts := &ManagerOptions{}
	mngOpts.apply(defaultManagerOptions...)
	mngOpts.apply(opts...)

	peeringManager := &Manager{
		Events: &ManagerEvents{
			Connect:            events.NewEvent(PeerCaller),
			Disconnect:         events.NewEvent(PeerCaller),
			Allowed:            events.NewEvent(PeerIDCaller),
			Disallowed:         events.NewEvent(PeerIDCaller),
			Connected:          events.NewEvent(PeerConnCaller),
			Disconnected:       events.NewEvent(PeerOptErrorCaller),
			ScheduledReconnect: events.NewEvent(PeerDurationCaller),
			Reconnecting:       events.NewEvent(PeerCaller),
			Reconnected:        events.NewEvent(PeerCaller),
			RelationUpdated:    events.NewEvent(PeerRelationCaller),
			StateChange:        events.NewEvent(ManagerStateCaller),
			Error:              events.NewEvent(events.ErrorCaller),
		},
		host:                   host,
		peers:                  map[peer.ID]*Peer{},
		allowedPeers:           map[peer.ID]struct{}{},
		opts:                   mngOpts,
		stopped:                typeutils.NewAtomicBool(),
		connectPeerChan:        make(chan *connectpeermsg, 10),
		connectPeerAttemptChan: make(chan *connectpeerattemptmsg, 10),
		reconnectChan:          make(chan *reconnectmsg, 100),
		reconnectAttemptChan:   make(chan *reconnectattemptmsg, 100),
		disconnectPeerChan:     make(chan *disconnectpeermsg, 10),
		isConnectedReqChan:     make(chan *isconnectedrequestmsg, 10),
		allowPeerChan:          make(chan *allowpeermsg, 10),
		disallowPeerChan:       make(chan *disallowpeermsg, 10),
		isAllowedReqChan:       make(chan *isallowedrequestmsg, 10),
		connectedChan:          make(chan *connectionmsg, 10),
		disconnectedChan:       make(chan *disconnectmsg, 10),
		forEachChan:            make(chan *foreachmsg, 10),
		callChan:               make(chan *callmsg, 10),
	}
	peeringManager.WrappedLogger = logger.NewWrappedLogger(peeringManager.opts.logger)
	peeringManager.configureEvents()

	return peeringManager
}

// Manager manages a set of known and other connected peers.
// It also provides the functionality to reconnect to known peers.
type Manager struct {
	// the logger used to log events.
	*logger.WrappedLogger

	// Events happening around the Manager.
	Events *ManagerEvents
	// the libp2p host instance from which to work with.
	host host.Host
	// holds the set of peers.
	peers map[peer.ID]*Peer
	// holds the set of allowed peers (autopeering).
	allowedPeers map[peer.ID]struct{}
	// holds the manager options.
	opts *ManagerOptions
	// tells whether the manager was shut down.
	stopped *typeutils.AtomicBool
	// event loop channels
	connectPeerChan        chan *connectpeermsg
	connectPeerAttemptChan chan *connectpeerattemptmsg
	reconnectChan          chan *reconnectmsg
	reconnectAttemptChan   chan *reconnectattemptmsg
	disconnectPeerChan     chan *disconnectpeermsg
	isConnectedReqChan     chan *isconnectedrequestmsg
	allowPeerChan          chan *allowpeermsg
	disallowPeerChan       chan *disallowpeermsg
	isAllowedReqChan       chan *isallowedrequestmsg
	connectedChan          chan *connectionmsg
	disconnectedChan       chan *disconnectmsg
	forEachChan            chan *foreachmsg
	callChan               chan *callmsg

	// closures.
	onP2PManagerConnect            *events.Closure
	onP2PManagerConnected          *events.Closure
	onP2PManagerDisconnect         *events.Closure
	onP2PManagerDisconnected       *events.Closure
	onP2PManagerScheduledReconnect *events.Closure
	onP2PManagerReconnecting       *events.Closure
	onP2PManagerRelationUpdated    *events.Closure
	onP2PManagerStateChange        *events.Closure
	onP2PManagerError              *events.Closure
}

// Start starts the Manager's event loop.
// This method blocks until the given context is done.
func (m *Manager) Start(ctx context.Context) {

	m.attachEvents()

	// manage libp2p network events
	m.host.Network().Notify((*netNotifiee)(m))

	m.Events.StateChange.Trigger(ManagerStateStarted)

	// run the event loop machinery
	m.eventLoop(ctx)

	m.Events.StateChange.Trigger(ManagerStateStopping)

	// close all connections
	for _, conn := range m.host.Network().Conns() {
		_ = conn.Close()
	}

	// de-register libp2p network events
	m.host.Network().StopNotify((*netNotifiee)(m))
	m.Events.StateChange.Trigger(ManagerStateStopped)

	m.detachEvents()
}

// shutdown sets the stopped flag and drains all outstanding requests of the event loop.
func (m *Manager) shutdown() {
	m.stopped.Set()

	// drain all outstanding requests of the event loop.
	// we don't care about correct handling of the channels, because we are shutting down anyway.
drainLoop:
	for {
		select {
		case connectPeerMsg := <-m.connectPeerChan:
			// do not connect to the peer
			connectPeerMsg.back <- ErrManagerShutdown

		case connectPeerAttemptMsg := <-m.connectPeerAttemptChan:
			// do not connect to the peer
			connectPeerAttemptMsg.back <- ErrManagerShutdown

		case <-m.reconnectChan:

		case <-m.reconnectAttemptChan:

		case disconnectPeerMsg := <-m.disconnectPeerChan:
			disconnectPeerMsg.back <- ErrManagerShutdown

		case isConnectedReqMsg := <-m.isConnectedReqChan:
			isConnectedReqMsg.back <- false

		case allowPeerMsg := <-m.allowPeerChan:
			allowPeerMsg.back <- ErrManagerShutdown

		case disallowPeerMsg := <-m.disallowPeerChan:
			disallowPeerMsg.back <- ErrManagerShutdown

		case isAllowedReqMsg := <-m.isAllowedReqChan:
			isAllowedReqMsg.back <- false

		case <-m.connectedChan:

		case <-m.disconnectedChan:

		case forEachMsg := <-m.forEachChan:
			forEachMsg.back <- struct{}{}

		case callMsg := <-m.callChan:
			callMsg.back <- struct{}{}

		default:
			break drainLoop
		}
	}
}

// ConnectPeer connects to the given peer.
// If the peer is considered "known" or "autopeered", then its connection is protected from trimming.
// Optionally an alias for the peer can be defined to better identify it afterwards.
func (m *Manager) ConnectPeer(addrInfo *peer.AddrInfo, peerRelation PeerRelation, alias ...string) error {
	if m.stopped.IsSet() {
		return ErrManagerShutdown
	}

	var al string
	if len(alias) > 0 {
		al = alias[0]
	}
	back := make(chan error)
	m.connectPeerChan <- &connectpeermsg{addrInfo: addrInfo, peerRelation: peerRelation, back: back, alias: al}

	return <-back
}

// DisconnectPeer disconnects the given peer.
// If the peer is considered "known", then its connection is unprotected from future trimming.
func (m *Manager) DisconnectPeer(peerID peer.ID, disconnectReason ...error) error {
	if m.stopped.IsSet() {
		return ErrManagerShutdown
	}

	back := make(chan error)
	var reason error
	if len(disconnectReason) > 0 {
		reason = disconnectReason[0]
	}
	m.disconnectPeerChan <- &disconnectpeermsg{peerID: peerID, reason: reason, back: back}

	return <-back
}

// IsConnected tells whether there is a connection to the given peer.
func (m *Manager) IsConnected(peerID peer.ID) bool {
	if m.stopped.IsSet() {
		return false
	}

	back := make(chan bool)
	m.isConnectedReqChan <- &isconnectedrequestmsg{peerID: peerID, back: back}

	return <-back
}

// AllowPeer allows incoming connections from the given peer (autopeering).
func (m *Manager) AllowPeer(peerID peer.ID) error {
	if m.stopped.IsSet() {
		return ErrManagerShutdown
	}

	back := make(chan error)
	m.allowPeerChan <- &allowpeermsg{peerID: peerID, back: back}

	return <-back
}

// DisallowPeer disallows incoming connections from the given peer (autopeering).
func (m *Manager) DisallowPeer(peerID peer.ID) error {
	if m.stopped.IsSet() {
		return ErrManagerShutdown
	}

	back := make(chan error)
	m.disallowPeerChan <- &disallowpeermsg{peerID: peerID, back: back}

	return <-back
}

// IsAllowed tells whether a connection to the given peer is allowed (autopeering).
func (m *Manager) IsAllowed(peerID peer.ID) bool {
	if m.stopped.IsSet() {
		return false
	}

	back := make(chan bool)
	m.isAllowedReqChan <- &isallowedrequestmsg{peerID: peerID, back: back}

	return <-back
}

// PeerForEachFunc is used in Manager.ForEach.
// Returning false indicates to stop looping.
// This function must not call any methods on Manager.
type PeerForEachFunc func(p *Peer) bool

// ForEach calls the given PeerForEachFunc on each Peer.
// Optionally only loops over the peers with the given filter relation.
func (m *Manager) ForEach(f PeerForEachFunc, filter ...PeerRelation) {
	if m.stopped.IsSet() {
		return
	}

	back := make(chan struct{})
	m.forEachChan <- &foreachmsg{f: f, back: back, filter: filter}
	<-back
}

// ConnectedCount returns the count of connected peer.
// Optionally only including peers with the given relation.
func (m *Manager) ConnectedCount(relation ...PeerRelation) int {
	var count int
	m.ForEach(func(p *Peer) bool {
		if m.host.Network().Connectedness(p.ID) == network.Connected {
			count++
		}

		return true
	}, relation...)

	return count
}

// PeerInfoSnapshot returns a snapshot of information of a peer with given id.
// If the peer is not known to the Manager, result is nil.
func (m *Manager) PeerInfoSnapshot(id peer.ID) *PeerInfoSnapshot {
	var info *PeerInfoSnapshot
	m.Call(id, func(p *Peer) {
		info = p.InfoSnapshot()
		info.Connected = m.host.Network().Connectedness(p.ID) == network.Connected
	})

	return info
}

// PeerInfoSnapshots returns snapshots of information of peers known to the Manager.
func (m *Manager) PeerInfoSnapshots() []*PeerInfoSnapshot {
	infos := make([]*PeerInfoSnapshot, 0)
	m.ForEach(func(p *Peer) bool {
		info := p.InfoSnapshot()
		info.Connected = m.host.Network().Connectedness(p.ID) == network.Connected
		infos = append(infos, info)

		return true
	})

	return infos
}

// PeerFunc gets called with the given Peer.
type PeerFunc func(p *Peer)

// Call calls the given PeerFunc synchronized within the Manager's event loop, if the peer exists.
// PeerFunc must not call any function on Manager.
func (m *Manager) Call(peerID peer.ID, f PeerFunc) {
	if m.stopped.IsSet() {
		return
	}

	back := make(chan struct{})
	m.callChan <- &callmsg{peerID: peerID, f: f, back: back}
	<-back
}

type connectpeermsg struct {
	addrInfo     *peer.AddrInfo
	peerRelation PeerRelation
	alias        string
	back         chan error
}

type connectpeerattemptmsg struct {
	addrInfo     *peer.AddrInfo
	peerRelation PeerRelation
	alias        string
	back         chan error
	connect      bool
	connectErr   error
}

type reconnectmsg struct {
	peerID peer.ID
}

type reconnectattemptmsg struct {
	peerID     peer.ID
	reconnect  bool
	connectErr error
}

type connectionmsg struct {
	net  network.Network
	conn network.Conn
}

type disconnectmsg struct {
	net    network.Network
	conn   network.Conn
	reason error
}

type disconnectpeermsg struct {
	peerID peer.ID
	reason error
	back   chan error
}

type isconnectedrequestmsg struct {
	peerID peer.ID
	back   chan bool
}

type allowpeermsg struct {
	peerID peer.ID
	back   chan error
}

type disallowpeermsg struct {
	peerID peer.ID
	back   chan error
}

type isallowedrequestmsg struct {
	peerID peer.ID
	back   chan bool
}

type foreachmsg struct {
	f      PeerForEachFunc
	back   chan struct{}
	filter []PeerRelation
}

type callmsg struct {
	peerID peer.ID
	f      PeerFunc
	back   chan struct{}
}

// runs the Manager's event loop, we do operations on the Manager in this way,
// because dealing with the natural concurrency of handling network connections
// becomes very messy, especially since libp2p's notifiee system isn't clear on
// what event is triggered when.
func (m *Manager) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			m.shutdown()

			return

		case connectPeerMsg := <-m.connectPeerChan:
			m.connectPeer(ctx, connectPeerMsg)

		case connectPeerAttemptMsg := <-m.connectPeerAttemptChan:
			if connectPeerAttemptMsg.connectErr != nil {
				if connectPeerAttemptMsg.connect {
					// unsuccessful connect:
					// get rid of the peer instance if the relation is unknown
					// or initiate a reconnect timer
					m.cleanupPeerIfNotKnown(connectPeerAttemptMsg.addrInfo.ID)
					m.scheduleReconnectIfKnown(connectPeerAttemptMsg.addrInfo.ID)
				}

				m.Events.Error.Trigger(fmt.Errorf("error connect to %s (%v): %w", connectPeerAttemptMsg.addrInfo.ID.ShortString(), connectPeerAttemptMsg.addrInfo.Addrs, connectPeerAttemptMsg.connectErr))

				if errors.Is(connectPeerAttemptMsg.connectErr, ErrPeerInManagerAlready) {
					m.updateRelation(connectPeerAttemptMsg.addrInfo.ID, connectPeerAttemptMsg.peerRelation)
					m.updateAlias(connectPeerAttemptMsg.addrInfo.ID, connectPeerAttemptMsg.alias)
				}
			}
			connectPeerAttemptMsg.back <- connectPeerAttemptMsg.connectErr

		case reconnectMsg := <-m.reconnectChan:
			m.reconnectPeer(ctx, reconnectMsg.peerID)

		case reconnectAttemptMsg := <-m.reconnectAttemptChan:
			if reconnectAttemptMsg.connectErr != nil {
				// unsuccessful connect:
				// get rid of the peer instance if the relation is unknown
				// or initiate a reconnect timer
				m.cleanupPeerIfNotKnown(reconnectAttemptMsg.peerID)
				m.scheduleReconnectIfKnown(reconnectAttemptMsg.peerID)

				m.Events.Error.Trigger(fmt.Errorf("error reconnect %s: %w", reconnectAttemptMsg.peerID.ShortString(), reconnectAttemptMsg.connectErr))

				continue
			}
			if !reconnectAttemptMsg.reconnect {
				continue
			}
			m.Events.Reconnected.Trigger(m.peers[reconnectAttemptMsg.peerID])

		case disconnectPeerMsg := <-m.disconnectPeerChan:
			p := m.peers[disconnectPeerMsg.peerID]
			disconnected, err := m.disconnectPeer(disconnectPeerMsg.peerID)
			if err != nil {
				m.Events.Error.Trigger(fmt.Errorf("error disconnect %s: %w", disconnectPeerMsg.peerID.ShortString(), err))
			}
			if disconnected {
				m.Events.Disconnected.Trigger(&PeerOptError{Peer: p, Error: disconnectPeerMsg.reason})
			}
			disconnectPeerMsg.back <- err

		case allowPeerMsg := <-m.allowPeerChan:
			err := m.allowPeer(allowPeerMsg.peerID)
			if err != nil {
				m.Events.Error.Trigger(fmt.Errorf("error allowing %s: %w", allowPeerMsg.peerID.ShortString(), err))
			}
			allowPeerMsg.back <- err

		case disallowPeerMsg := <-m.disallowPeerChan:
			m.disallowPeer(disallowPeerMsg.peerID)
			disallowPeerMsg.back <- nil

		case isAllowedReqMsg := <-m.isAllowedReqChan:
			allowed := m.isAllowed(isAllowedReqMsg.peerID)
			isAllowedReqMsg.back <- allowed

		case isConnectedReqMsg := <-m.isConnectedReqChan:
			connected := m.isConnected(isConnectedReqMsg.peerID)
			isConnectedReqMsg.back <- connected

		case connectedMsg := <-m.connectedChan:
			p := m.peers[connectedMsg.conn.RemotePeer()]
			m.addPeerAsUnknownIfAbsent(connectedMsg.conn)
			if p != nil {
				m.resetReconnect(p.ID)
				if !p.connectedEventCalled {
					m.Events.Connected.Trigger(p, connectedMsg.conn)
					p.connectedEventCalled = true
				}
			}

		case disconnectedMsg := <-m.disconnectedChan:
			id := disconnectedMsg.conn.RemotePeer()
			p := m.peers[id]

			// nothing to do if we're still connected
			if m.host.Network().Connectedness(id) == network.Connected {
				continue
			}

			m.cleanupPeerIfNotKnown(id)
			m.scheduleReconnectIfKnown(id)
			if p != nil {
				m.Events.Disconnected.Trigger(&PeerOptError{Peer: p, Error: disconnectedMsg.reason})
			}

		case forEachMsg := <-m.forEachChan:
			m.forEach(forEachMsg.f, forEachMsg.filter...)
			forEachMsg.back <- struct{}{}

		case callMsg := <-m.callChan:
			m.call(callMsg.peerID, callMsg.f)
			callMsg.back <- struct{}{}
		}
	}
}

// connects to the given peer if it isn't already connected and if its relation is PeerRelationKnown,
// then the connection to the peer is further protected from trimming.
func (m *Manager) connectPeer(ctx context.Context, connectPeerMsg *connectpeermsg) {

	if _, has := m.peers[connectPeerMsg.addrInfo.ID]; has {
		m.connectPeerAttemptChan <- &connectpeerattemptmsg{
			addrInfo:     connectPeerMsg.addrInfo,
			peerRelation: connectPeerMsg.peerRelation,
			alias:        connectPeerMsg.alias,
			// pass the error channel of the caller to the connectPeerAttemptChan
			back:       connectPeerMsg.back,
			connect:    false,
			connectErr: ErrPeerInManagerAlready,
		}

		return
	}

	if connectPeerMsg.addrInfo.ID == m.host.ID() {
		m.connectPeerAttemptChan <- &connectpeerattemptmsg{
			addrInfo:     connectPeerMsg.addrInfo,
			peerRelation: connectPeerMsg.peerRelation,
			alias:        connectPeerMsg.alias,
			// pass the error channel of the caller to the connectPeerAttemptChan
			back:       connectPeerMsg.back,
			connect:    false,
			connectErr: ErrCantConnectToItself,
		}

		return
	}

	p := NewPeer(connectPeerMsg.addrInfo.ID, connectPeerMsg.peerRelation, connectPeerMsg.addrInfo.Addrs, connectPeerMsg.alias)
	if p.Relation == PeerRelationKnown || p.Relation == PeerRelationAutopeered {
		m.host.ConnManager().Protect(connectPeerMsg.addrInfo.ID, PeerConnectivityProtectionTag)
	}

	m.peers[connectPeerMsg.addrInfo.ID] = p
	m.Events.Connect.Trigger(p)

	// perform an actual connection attempt to the given peer.
	// connection attempts should happen in a separate goroutine
	go func() {
		ctxConnect, cancelConnect := context.WithTimeout(ctx, connTimeout)
		defer cancelConnect()

		// if the connection fails, the peer is either cleared from the Manager if its relation is PeerRelationUnknown
		// or a reconnect attempt is scheduled if it is PeerRelationKnown.
		// this is done in via the connectPeerAttemptChan.
		m.connectPeerAttemptChan <- &connectpeerattemptmsg{
			addrInfo:     connectPeerMsg.addrInfo,
			peerRelation: connectPeerMsg.peerRelation,
			alias:        connectPeerMsg.alias,
			// pass the error channel of the caller to the connectPeerAttemptChan
			back:       connectPeerMsg.back,
			connect:    true,
			connectErr: m.host.Connect(ctxConnect, *connectPeerMsg.addrInfo),
		}
	}()
}

// reconnect peer does a connection attempt to the given peer but only
// if its relation is PeerRelationKnown.
func (m *Manager) reconnectPeer(ctx context.Context, peerID peer.ID) {
	p, has := m.peers[peerID]
	if !has {
		// directly return the result of the reconnect attempt
		m.reconnectAttemptChan <- &reconnectattemptmsg{
			peerID:     peerID,
			reconnect:  false,
			connectErr: nil,
		}

		return
	}

	if p.Relation != PeerRelationKnown || p.reconnectTimer == nil {
		// directly return the result of the reconnect attempt
		m.reconnectAttemptChan <- &reconnectattemptmsg{
			peerID:     peerID,
			reconnect:  false,
			connectErr: nil,
		}

		return
	}

	m.Events.Reconnecting.Trigger(p)

	addrInfo := peer.AddrInfo{ID: peerID, Addrs: p.Addrs}

	// perform an actual connection attempt to the given peer.
	// connection attempts should happen in a separate goroutine
	go func() {
		ctxConnect, cancelConnect := context.WithTimeout(ctx, connTimeout)
		defer cancelConnect()

		// if the connection fails, the peer is either cleared from the Manager if its relation is PeerRelationUnknown
		// or a reconnect attempt is scheduled if it is PeerRelationKnown.
		// this is done in via the reconnectAttemptChan.
		m.reconnectAttemptChan <- &reconnectattemptmsg{
			peerID:     peerID,
			reconnect:  true,
			connectErr: m.host.Connect(ctxConnect, addrInfo),
		}
	}()
}

// disconnects and removes the given peer from the Manager.
// also clears the protection state of the peer.
func (m *Manager) disconnectPeer(peerID peer.ID) (bool, error) {
	p, has := m.peers[peerID]
	if !has {
		return false, nil
	}
	m.host.ConnManager().Unprotect(peerID, PeerConnectivityProtectionTag)
	delete(m.peers, peerID)
	m.Events.Disconnect.Trigger(p)

	return true, m.host.Network().ClosePeer(peerID)
}

// allows incoming connections from the given peer (autopeering).
func (m *Manager) allowPeer(peerID peer.ID) error {
	if _, has := m.allowedPeers[peerID]; has {
		return ErrPeerInManagerAlreadyAllowed
	}

	if peerID == m.host.ID() {
		return ErrCantAllowItself
	}

	m.allowedPeers[peerID] = struct{}{}
	m.Events.Allowed.Trigger(peerID)

	return nil
}

// disallows incoming connections from the given peer (autopeering).
func (m *Manager) disallowPeer(peerID peer.ID) {
	if _, has := m.allowedPeers[peerID]; !has {
		return
	}

	delete(m.allowedPeers, peerID)
	m.Events.Disallowed.Trigger(peerID)
}

// checks whether the given peer is allowed to connect (autopeering).
func (m *Manager) isAllowed(peerID peer.ID) bool {
	_, has := m.allowedPeers[peerID]

	return has
}

// updates the relation to the given peer to the given new relation.
// if the new relation is PeerRelationKnown, then the peer will be protected from trimming.
func (m *Manager) updateRelation(peerID peer.ID, newRelation PeerRelation) {
	p := m.peers[peerID]
	if p.Relation == newRelation {
		return
	}
	oldRelation := p.Relation
	p.Relation = newRelation

	switch newRelation {
	case PeerRelationUnknown:
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil

	case PeerRelationAutopeered:
		fallthrough

	case PeerRelationKnown:
		m.host.ConnManager().Protect(peerID, PeerConnectivityProtectionTag)
	}

	m.Events.RelationUpdated.Trigger(p, oldRelation)
}

// updates the alias of the given peer but only if it is empty.
func (m *Manager) updateAlias(peerID peer.ID, alias string) {
	p, has := m.peers[peerID]
	if !has {
		return
	}
	if len(p.Alias) != 0 {
		return
	}
	p.Alias = alias
}

// schedules the reconnect timer for a reconnect attempt to the given peer,
// if the peer's relation is PeerRelationKnown.
func (m *Manager) scheduleReconnectIfKnown(peerID peer.ID) {
	p, has := m.peers[peerID]
	if !has {
		return
	}

	if p.Relation != PeerRelationKnown {
		return
	}

	if p.reconnectTimer != nil {
		p.reconnectTimer.Stop()
	}
	p.connectedEventCalled = false

	delay := m.opts.reconnectDelay()
	p.reconnectTimer = time.AfterFunc(delay, func() {
		if m.stopped.IsSet() {
			return
		}

		m.reconnectChan <- &reconnectmsg{peerID: peerID}
	})
	m.Events.ScheduledReconnect.Trigger(p, delay)
}

// resets the reconnect timer for the given peer.
func (m *Manager) resetReconnect(peerID peer.ID) {
	p, has := m.peers[peerID]
	if !has {
		return
	}
	if p.reconnectTimer != nil {
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil
		// we reconnected a peer which was scheduled for a reconnect
		m.Events.Reconnected.Trigger(p)
	}
}

// adds the given connection as peer with PeerRelationUnknown to the Manager's peer set,
// if the peer isn't already in the set.
func (m *Manager) addPeerAsUnknownIfAbsent(conn network.Conn) {
	if conn.Stat().Direction == network.DirOutbound {
		return
	}

	if _, has := m.peers[conn.RemotePeer()]; !has {
		// add unknown peer to manager
		addrs := []multiaddr.Multiaddr{conn.RemoteMultiaddr()}

		relation := PeerRelationUnknown
		if m.isAllowed(conn.RemotePeer()) {
			relation = PeerRelationAutopeered
		}
		m.peers[conn.RemotePeer()] = NewPeer(conn.RemotePeer(), relation, addrs, "")
	}
}

// removes a not known peer if it has no more connections.
func (m *Manager) cleanupPeerIfNotKnown(peerID peer.ID) {
	p, has := m.peers[peerID]
	if has && p.Relation != PeerRelationKnown && len(m.host.Network().ConnsToPeer(peerID)) == 0 {
		m.host.ConnManager().Unprotect(peerID, PeerConnectivityProtectionTag)
		delete(m.peers, peerID)
	}
}

// checks whether the given peer is connected.
func (m *Manager) isConnected(peerID peer.ID) bool {
	if _, has := m.peers[peerID]; !has {
		return false
	}

	return m.host.Network().Connectedness(peerID) == network.Connected
}

// calls the given PeerForEachFunc on each Peer.
func (m *Manager) forEach(f PeerForEachFunc, filter ...PeerRelation) {
	for _, p := range m.peers {
		if len(filter) > 0 && p.Relation != filter[0] {
			continue
		}
		if m.stopped.IsSet() || !f(p) {
			break
		}
	}
}

// calls the given PeerFunc if the given peer exists.
func (m *Manager) call(peerID peer.ID, f PeerFunc) {
	p, has := m.peers[peerID]
	if !has {
		return
	}
	f(p)
}

func (m *Manager) configureEvents() {

	// logger
	m.onP2PManagerConnect = events.NewClosure(func(p *Peer) {
		m.LogInfof("connecting %s: %s", p.ID.ShortString(), p.Addrs)
	})

	m.onP2PManagerConnected = events.NewClosure(func(p *Peer, conn network.Conn) {
		m.LogInfof("connected %s (%s)", p.ID.ShortString(), conn.Stat().Direction.String())
	})

	m.onP2PManagerDisconnect = events.NewClosure(func(p *Peer) {
		m.LogInfof("disconnecting %s", p.ID.ShortString())
	})

	m.onP2PManagerDisconnected = events.NewClosure(func(peerErr *PeerOptError) {
		msg := fmt.Sprintf("disconnected %s", peerErr.Peer.ID.ShortString())
		if peerErr.Error != nil {
			msg = fmt.Sprintf("%s %s", msg, peerErr.Error)
		}
		m.LogInfof(msg)
	})

	m.onP2PManagerScheduledReconnect = events.NewClosure(func(p *Peer, dur time.Duration) {
		m.LogInfof("scheduled reconnect in %v to %s", dur, p.ID.ShortString())
	})

	m.onP2PManagerReconnecting = events.NewClosure(func(p *Peer) {
		m.LogInfof("reconnecting %s", p.ID.ShortString())
	})

	m.onP2PManagerRelationUpdated = events.NewClosure(func(p *Peer, oldRel PeerRelation) {
		m.LogInfof("updated relation of %s from '%s' to '%s'", p.ID.ShortString(), oldRel, p.Relation)
	})

	m.onP2PManagerStateChange = events.NewClosure(func(mngState ManagerState) {
		m.LogInfo(mngState)
	})

	m.onP2PManagerError = events.NewClosure(func(err error) {
		m.LogWarn(err)
	})
}

func (m *Manager) attachEvents() {
	m.Events.Connect.Hook(m.onP2PManagerConnect)
	m.Events.Connected.Hook(m.onP2PManagerConnected)
	m.Events.Disconnect.Hook(m.onP2PManagerDisconnect)
	m.Events.Disconnected.Hook(m.onP2PManagerDisconnected)
	m.Events.ScheduledReconnect.Hook(m.onP2PManagerScheduledReconnect)
	m.Events.Reconnecting.Hook(m.onP2PManagerReconnecting)
	m.Events.RelationUpdated.Hook(m.onP2PManagerRelationUpdated)
	m.Events.StateChange.Hook(m.onP2PManagerStateChange)
	m.Events.Error.Hook(m.onP2PManagerError)
}

func (m *Manager) detachEvents() {
	m.Events.Connect.Detach(m.onP2PManagerConnect)
	m.Events.Connected.Detach(m.onP2PManagerConnected)
	m.Events.Disconnect.Detach(m.onP2PManagerDisconnect)
	m.Events.Disconnected.Detach(m.onP2PManagerDisconnected)
	m.Events.ScheduledReconnect.Detach(m.onP2PManagerScheduledReconnect)
	m.Events.Reconnecting.Detach(m.onP2PManagerReconnecting)
	m.Events.RelationUpdated.Detach(m.onP2PManagerRelationUpdated)
	m.Events.StateChange.Detach(m.onP2PManagerStateChange)
	m.Events.Error.Detach(m.onP2PManagerError)
}

// lets Manager implement network.Notifiee, we do this as a separate
// type to not pollute the Manager's public methods.
// the handlers are called in newly spawned goroutines.
type netNotifiee Manager

func (m *netNotifiee) Listen(_ network.Network, _ multiaddr.Multiaddr)      {}
func (m *netNotifiee) ListenClose(_ network.Network, _ multiaddr.Multiaddr) {}
func (m *netNotifiee) Connected(net network.Network, conn network.Conn) {
	if m.stopped.IsSet() {
		return
	}
	m.connectedChan <- &connectionmsg{net: net, conn: conn}
}
func (m *netNotifiee) Disconnected(net network.Network, conn network.Conn) {
	if m.stopped.IsSet() {
		return
	}
	m.disconnectedChan <- &disconnectmsg{net: net, conn: conn, reason: errors.New("connection closed by libp2p network event")}
}
