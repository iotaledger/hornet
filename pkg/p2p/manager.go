package p2p

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/typeutils"
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
	handler.(func(*Peer))(params[0].(*Peer))
}

// PeerOptError holds a Peer and optionally an error.
type PeerOptError struct {
	Peer  *Peer
	Error error
}

// PeerOptErrorCaller gets called with a Peer and an error.
func PeerOptErrorCaller(handler interface{}, params ...interface{}) {
	handler.(func(*PeerOptError))(params[0].(*PeerOptError))
}

// ManagerStateCaller gets called with a ManagerState.
func ManagerStateCaller(handler interface{}, params ...interface{}) {
	handler.(func(ManagerState))(params[0].(ManagerState))
}

// PeerDurationCaller gets called with a Peer and a time.Duration.
func PeerDurationCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Peer, time.Duration))(params[0].(*Peer), params[1].(time.Duration))
}

// PeerConnCaller gets called with a Peer and its associated network.Conn.
func PeerConnCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Peer, network.Conn))(params[0].(*Peer), params[1].(network.Conn))
}

// PeerRelationCaller gets called with a Peer and its old PeerRelation.
func PeerRelationCaller(handler interface{}, params ...interface{}) {
	handler.(func(p *Peer, old PeerRelation))(params[0].(*Peer), params[1].(PeerRelation))
}

// the default options applied to the Manager.
var defaultManagerOptions = []ManagerOption{
	WithManagerReconnectInterval(30*time.Second, 1*time.Second),
}

// ManagerOptions define options for a Manager.
type ManagerOptions struct {
	// The logger to use to log events.
	Logger *logger.Logger
	// The static reconnect interval.
	ReconnectInterval time.Duration
	// The randomized jitter applied to the reconnect interval.
	ReconnectIntervalJitter time.Duration
}

// ManagerOption is a function setting a ManagerOptions option.
type ManagerOption func(opts *ManagerOptions)

// WithManagerLogger enables logging within the Manager.
func WithManagerLogger(logger *logger.Logger) ManagerOption {
	return func(opts *ManagerOptions) {
		opts.Logger = logger
	}
}

// WithManagerReconnectInterval defines the re-connect interval for peers
// to which the Manager wants to keep a connection open to.
func WithManagerReconnectInterval(interval time.Duration, jitter time.Duration) ManagerOption {
	return func(opts *ManagerOptions) {
		opts.ReconnectInterval = interval
		opts.ReconnectIntervalJitter = jitter
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
	recInter := mo.ReconnectInterval
	jitter := mo.ReconnectIntervalJitter
	delayJitter := rand.Int63n(int64(jitter))
	return recInter + time.Duration(delayJitter)
}

// NewManager creates a new Manager.
func NewManager(host host.Host, opts ...ManagerOption) *Manager {
	mngOpts := &ManagerOptions{}
	mngOpts.apply(defaultManagerOptions...)
	mngOpts.apply(opts...)

	peeringManager := &Manager{
		Events: ManagerEvents{
			Connect:            events.NewEvent(PeerCaller),
			Disconnect:         events.NewEvent(PeerCaller),
			Connected:          events.NewEvent(PeerConnCaller),
			Disconnected:       events.NewEvent(PeerOptErrorCaller),
			ScheduledReconnect: events.NewEvent(PeerDurationCaller),
			Reconnecting:       events.NewEvent(PeerCaller),
			Reconnected:        events.NewEvent(PeerCaller),
			RelationUpdated:    events.NewEvent(PeerRelationCaller),
			StateChange:        events.NewEvent(ManagerStateCaller),
			Error:              events.NewEvent(events.ErrorCaller),
		},
		host:               host,
		peers:              map[peer.ID]*Peer{},
		opts:               mngOpts,
		stopped:            typeutils.NewAtomicBool(),
		connectPeerChan:    make(chan *connectpeermsg, 10),
		disconnectPeerChan: make(chan *disconnectpeermsg, 10),
		isConnectedReqChan: make(chan *isconnectedrequestmsg, 10),
		connectedChan:      make(chan *connectionmsg, 10),
		disconnectedChan:   make(chan *disconnectmsg, 10),
		reconnectChan:      make(chan *reconnectmsg, 100),
		forEachChan:        make(chan *foreachmsg, 10),
		callChan:           make(chan *callmsg, 10),
	}
	if peeringManager.opts.Logger != nil {
		peeringManager.registerLoggerOnEvents()
	}
	return peeringManager
}

// Manager manages a set of known and other connected peers.
// It also provides the functionality to reconnect to known peers.
type Manager struct {
	// Events happening around the Manager.
	Events ManagerEvents
	// the libp2p host instance from which to work with.
	host host.Host
	// holds the set of peers.
	peers map[peer.ID]*Peer
	// holds the manager options.
	opts *ManagerOptions
	// tells whether the manager was shut down.
	stopped *typeutils.AtomicBool
	// event loop channels
	connectPeerChan    chan *connectpeermsg
	disconnectPeerChan chan *disconnectpeermsg
	isConnectedReqChan chan *isconnectedrequestmsg
	connectedChan      chan *connectionmsg
	disconnectedChan   chan *disconnectmsg
	reconnectChan      chan *reconnectmsg
	forEachChan        chan *foreachmsg
	callChan           chan *callmsg
}

// Start starts the Manager's event loop.
// This method blocks until shutdownSignal has been signaled.
func (m *Manager) Start(shutdownSignal <-chan struct{}) {
	// manage libp2p network events
	m.host.Network().Notify((*netNotifiee)(m))

	m.Events.StateChange.Trigger(ManagerStateStarted)

	// run the event loop machinery
	m.eventLoop(shutdownSignal)

	m.Events.StateChange.Trigger(ManagerStateStopping)

	// close all connections
	for _, conn := range m.host.Network().Conns() {
		_ = conn.Close()
	}

	// de-register libp2p network events
	m.host.Network().StopNotify((*netNotifiee)(m))
	m.Events.StateChange.Trigger(ManagerStateStopped)
}

// shutdown sets the stopped flag and drains all outstanding requests of the event loop.
func (m *Manager) shutdown() {
	m.stopped.Set()

	// drain all outstanding requests of the event loop.
	// we do not care about correct handling of the channels, because we are shutting down anyway.
drainLoop:
	for {
		select {
		case connectPeerMsg := <-m.connectPeerChan:
			// do not connect to the peer
			connectPeerMsg.back <- ErrManagerShutdown

		case disconnectPeerMsg := <-m.disconnectPeerChan:
			disconnectPeerMsg.back <- ErrManagerShutdown

		case <-m.reconnectChan:

		case isConnectedReqMsg := <-m.isConnectedReqChan:
			isConnectedReqMsg.back <- false

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

type reconnectmsg struct {
	peerID peer.ID
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
func (m *Manager) eventLoop(shutdownSignal <-chan struct{}) {
	for {
		select {
		case <-shutdownSignal:
			m.shutdown()
			return

		case connectPeerMsg := <-m.connectPeerChan:
			err := m.connectPeer(connectPeerMsg.addrInfo, connectPeerMsg.peerRelation, connectPeerMsg.alias)
			if err != nil {
				m.Events.Error.Trigger(fmt.Errorf("error connect to %s (%v): %w", connectPeerMsg.addrInfo.ID.ShortString(), connectPeerMsg.addrInfo.Addrs, err))
			}
			if errors.Is(err, ErrPeerInManagerAlready) {
				m.updateRelation(connectPeerMsg.addrInfo.ID, connectPeerMsg.peerRelation)
				m.updateAlias(connectPeerMsg.addrInfo.ID, connectPeerMsg.alias)
			}
			connectPeerMsg.back <- err

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

		case reconnectMsg := <-m.reconnectChan:
			reconnect, err := m.reconnectPeer(reconnectMsg.peerID)
			if err != nil {
				m.Events.Error.Trigger(fmt.Errorf("error reconnect %s: %w", reconnectMsg.peerID.ShortString(), err))
				continue
			}
			if !reconnect {
				continue
			}
			m.Events.Reconnected.Trigger(m.peers[reconnectMsg.peerID])

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
func (m *Manager) connectPeer(addrInfo *peer.AddrInfo, relation PeerRelation, alias string) error {
	if _, has := m.peers[addrInfo.ID]; has {
		return ErrPeerInManagerAlready
	}

	if addrInfo.ID == m.host.ID() {
		return ErrCantConnectToItself
	}

	p := NewPeer(addrInfo.ID, relation, addrInfo.Addrs, alias)
	if p.Relation == PeerRelationKnown || p.Relation == PeerRelationAutopeered {
		m.host.ConnManager().Protect(addrInfo.ID, PeerConnectivityProtectionTag)
	}

	m.peers[addrInfo.ID] = p
	m.Events.Connect.Trigger(p)

	return m.connect(*addrInfo)
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

// reconnect peer does a connection attempt to the given peer but only
// if its relation is PeerRelationKnown.
func (m *Manager) reconnectPeer(peerID peer.ID) (bool, error) {
	p, has := m.peers[peerID]
	if !has {
		return false, nil
	}

	if p.Relation != PeerRelationKnown || p.reconnectTimer == nil {
		return false, nil
	}

	m.Events.Reconnecting.Trigger(p)
	return true, m.connect(peer.AddrInfo{ID: peerID, Addrs: p.Addrs})
}

// connect does an actual connection attempt to the given peer.
// if the connection fails, the peer is either cleared from the Manager if its relation is PeerRelationUnknown
// or a reconnect attempt is scheduled if it is PeerRelationKnown.
func (m *Manager) connect(addrInfo peer.AddrInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
	defer cancel()

	err := m.host.Connect(ctx, addrInfo)
	if err != nil {
		// unsuccessful connect:
		// get rid of the peer instance if the relation is unknown
		// or initiate a reconnect timer
		m.cleanupPeerIfNotKnown(addrInfo.ID)
		m.scheduleReconnectIfKnown(addrInfo.ID)
	}
	return err
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
		m.peers[conn.RemotePeer()] = NewPeer(conn.RemotePeer(), PeerRelationUnknown, addrs, "")
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

// registers the logger on the events of the Manager.
func (m *Manager) registerLoggerOnEvents() {
	m.Events.Connect.Attach(events.NewClosure(func(p *Peer) {
		m.opts.Logger.Infof("connecting %s: %s", p.ID.ShortString(), p.Addrs)
	}))
	m.Events.Connected.Attach(events.NewClosure(func(p *Peer, conn network.Conn) {
		m.opts.Logger.Infof("connected %s (%s)", p.ID.ShortString(), conn.Stat().Direction.String())
	}))
	m.Events.Disconnect.Attach(events.NewClosure(func(p *Peer) {
		m.opts.Logger.Infof("disconnecting %s", p.ID.ShortString())
	}))
	m.Events.Disconnected.Attach(events.NewClosure(func(peerErr *PeerOptError) {
		msg := fmt.Sprintf("disconnected %s", peerErr.Peer.ID.ShortString())
		if peerErr.Error != nil {
			msg = fmt.Sprintf("%s %s", msg, peerErr.Error)
		}
		m.opts.Logger.Infof(msg)
	}))
	m.Events.ScheduledReconnect.Attach(events.NewClosure(func(p *Peer, dur time.Duration) {
		m.opts.Logger.Infof("scheduled reconnect in %v to %s", dur, p.ID.ShortString())
	}))
	m.Events.Reconnecting.Attach(events.NewClosure(func(p *Peer) {
		m.opts.Logger.Infof("reconnecting %s", p.ID.ShortString())
	}))
	m.Events.RelationUpdated.Attach(events.NewClosure(func(p *Peer, oldRel PeerRelation) {
		m.opts.Logger.Infof("updated relation of %s from '%s' to '%s'", p.ID.ShortString(), oldRel, p.Relation)
	}))
	m.Events.StateChange.Attach(events.NewClosure(func(mngState ManagerState) {
		m.opts.Logger.Info(mngState)
	}))
	m.Events.Error.Attach(events.NewClosure(func(err error) {
		m.opts.Logger.Warn(err)
	}))
}

// lets Manager implement network.Notifiee, we do this as a separate
// type to not pollute the Manager's public methods.
// the handlers are called in newly spawned goroutines.
type netNotifiee Manager

func (m *netNotifiee) Listen(net network.Network, multiaddr multiaddr.Multiaddr)      {}
func (m *netNotifiee) ListenClose(net network.Network, multiaddr multiaddr.Multiaddr) {}
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
func (m *netNotifiee) OpenedStream(net network.Network, stream network.Stream) {}
func (m *netNotifiee) ClosedStream(net network.Network, stream network.Stream) {}
