package p2p

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	// KnownPeerConnectivityProtectionTag is the tag used by Manager to
	// protect known peer connectivity from getting trimmed via the
	// connmgr.ConnManager.
	KnownPeerConnectivityProtectionTag = "peering-manager"

	connTimeout = 2 * time.Second
)

var (
	// Returned if the manager is supposed to create a connection
	// to itself (host wise).
	ErrCantConnectToItself = errors.New("the host can't connect to itself")
	// Returned if a peer is tried to be added to the manager which is already added.
	ErrPeerInManagerAlready = errors.New("peer is already in manager")
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
	// PeerRelationDiscovered is a relation to a discovered peer.
	// Connections to such peers do not have to be retained.
	PeerRelationDiscovered PeerRelation = "discovered"
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
	// Fired when internal error happens.
	Error *events.Event
}

// PeerCaller gets called with a Peer.
func PeerCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Peer))(params[0].(*Peer))
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
	Logger                  *logger.Logger
	ReconnectInterval       time.Duration
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

// applies the given ServiceOption.
func (mo *ManagerOptions) apply(opts ...ManagerOption) {
	mo.Logger = nil
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
	mng := &Manager{
		Events: ManagerEvents{
			Connect:            events.NewEvent(PeerCaller),
			Disconnect:         events.NewEvent(PeerCaller),
			Connected:          events.NewEvent(PeerConnCaller),
			Disconnected:       events.NewEvent(PeerCaller),
			ScheduledReconnect: events.NewEvent(PeerDurationCaller),
			Reconnecting:       events.NewEvent(PeerCaller),
			Reconnected:        events.NewEvent(PeerCaller),
			RelationUpdated:    events.NewEvent(PeerRelationCaller),
			Error:              events.NewEvent(events.ErrorCaller),
		},
		host:               host,
		peers:              map[peer.ID]*Peer{},
		opts:               mngOpts,
		connectPeerChan:    make(chan *connectpeermsg),
		disconnectPeerChan: make(chan *disconnectpeermsg),
		isConnectedReqChan: make(chan *isconnectedrequestmsg),
		connectedChan:      make(chan *connectionmsg),
		disconnectedChan:   make(chan *connectionmsg),
		reconnectChan:      make(chan *reconnectmsg, 100),
		forEachChan:        make(chan *foreachmsg),
		callChan:           make(chan *callmsg),
	}
	if mng.opts.Logger != nil {
		mng.registerLoggerOnEvents()
	}
	return mng
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
	opts  *ManagerOptions
	// event loop channels
	connectPeerChan    chan *connectpeermsg
	disconnectPeerChan chan *disconnectpeermsg
	isConnectedReqChan chan *isconnectedrequestmsg
	connectedChan      chan *connectionmsg
	disconnectedChan   chan *connectionmsg
	reconnectChan      chan *reconnectmsg
	forEachChan        chan *foreachmsg
	callChan           chan *callmsg
}

// Start starts the Manager's event loop.
// This method blocks until shutdownSignal has been signaled.
func (m *Manager) Start(shutdownSignal <-chan struct{}) {
	// manage libp2p network events
	m.host.Network().Notify((*netNotifiee)(m))
	// run the event loop machinery
	m.eventLoop(shutdownSignal)

	for _, conn := range m.host.Network().Conns() {
		_ = conn.Close()
	}

	// de-register libp2p network events
	m.host.Network().StopNotify((*netNotifiee)(m))
}

// ConnectPeer connects to the given peer.
// If the peer is considered "known", then its connection is protected from trimming.
// Optionally an alias for the peer can be defined to better identify it afterwards.
func (m *Manager) ConnectPeer(addrInfo *peer.AddrInfo, peerRelation PeerRelation, alias ...string) error {
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
func (m *Manager) DisconnectPeer(id peer.ID) error {
	back := make(chan error)
	m.disconnectPeerChan <- &disconnectpeermsg{id: id, back: back}
	return <-back
}

// IsConnected tells whether there is a connection to the given peer.
func (m *Manager) IsConnected(id peer.ID) bool {
	back := make(chan bool)
	m.isConnectedReqChan <- &isconnectedrequestmsg{id: id, back: back}
	return <-back
}

// PeerForEachFunc is used in Manager.ForEach.
// Returning false indicates to stop looping.
// This function must not call any methods on Manager.
type PeerForEachFunc func(p *Peer) bool

// ForEach calls the given PeerForEachFunc on each Peer.
// Optionally only loops over the peers with the given filter relation.
func (m *Manager) ForEach(f PeerForEachFunc, filter ...PeerRelation) {
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

// PeerSnapshots returns snapshots of information of peers known to the Manager.
func (m *Manager) PeerSnapshots() []*PeerSnapshot {
	infos := make([]*PeerSnapshot, 0)
	m.ForEach(func(p *Peer) bool {
		info := p.Info()
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
func (m *Manager) Call(id peer.ID, f PeerFunc) {
	back := make(chan struct{})
	m.callChan <- &callmsg{id: id, f: f, back: back}
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

type disconnectpeermsg struct {
	id   peer.ID
	back chan error
}

type isconnectedrequestmsg struct {
	id   peer.ID
	back chan bool
}

type reconnectmsg struct {
	id peer.ID
}

type foreachmsg struct {
	f      PeerForEachFunc
	back   chan struct{}
	filter []PeerRelation
}

type callmsg struct {
	id   peer.ID
	f    PeerFunc
	back chan struct{}
}

// runs the Manager's event loop, we do operations on the Manager in this way,
// because dealing with the natural concurrency of handling network connections
// becomes very messy, especially since libp2p's notifiee system isn't clear on
// what event is triggered when.
func (m *Manager) eventLoop(shutdownSignal <-chan struct{}) {
	for {
		select {
		case <-shutdownSignal:
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
			p := m.peers[disconnectPeerMsg.id]
			disconnected, err := m.disconnectPeer(disconnectPeerMsg.id)
			if err != nil {
				m.Events.Error.Trigger(fmt.Errorf("error disconnect %s: %w", disconnectPeerMsg.id.ShortString(), err))
			}
			if disconnected {
				m.Events.Disconnected.Trigger(p)
			}
			disconnectPeerMsg.back <- err

		case reconnectMsg := <-m.reconnectChan:
			reconnect, err := m.reconnectPeer(reconnectMsg.id)
			if err != nil {
				m.Events.Error.Trigger(fmt.Errorf("error reconnect %s: %w", reconnectMsg.id.ShortString(), err))
				continue
			}
			if !reconnect {
				continue
			}
			m.Events.Reconnected.Trigger(m.peers[reconnectMsg.id])

		case isConnectedReqMsg := <-m.isConnectedReqChan:
			connected := m.isConnected(isConnectedReqMsg.id)
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

			m.cleanupPeerIfUnknown(id)
			m.scheduleReconnectIfKnown(id)
			if p != nil {
				m.Events.Disconnected.Trigger(p)
			}

		case forEachMsg := <-m.forEachChan:
			m.forEach(forEachMsg.f, forEachMsg.filter...)
			forEachMsg.back <- struct{}{}

		case callMsg := <-m.callChan:
			m.call(callMsg.id, callMsg.f)
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
	if p.Relation == PeerRelationKnown {
		m.host.ConnManager().Protect(addrInfo.ID, KnownPeerConnectivityProtectionTag)
	}

	m.peers[addrInfo.ID] = p
	m.Events.Connect.Trigger(p)

	return m.connect(*addrInfo)
}

// disconnects and removes the given peer from the Manager.
// also clears the protection state of the peer.
func (m *Manager) disconnectPeer(id peer.ID) (bool, error) {
	p, has := m.peers[id]
	if !has {
		return false, nil
	}
	m.host.ConnManager().Unprotect(id, KnownPeerConnectivityProtectionTag)
	delete(m.peers, id)
	m.Events.Disconnect.Trigger(p)
	return true, m.host.Network().ClosePeer(id)
}

// updates the relation to the given peer to the given new relation.
// if the new relation is PeerRelationKnown, then the peer will be protected from trimming.
func (m *Manager) updateRelation(id peer.ID, newRelation PeerRelation) {
	p := m.peers[id]
	if p.Relation == newRelation {
		return
	}
	oldRelation := p.Relation
	p.Relation = newRelation
	switch newRelation {
	case PeerRelationUnknown:
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil
	case PeerRelationKnown:
		m.host.ConnManager().Protect(id, KnownPeerConnectivityProtectionTag)
	}
	m.Events.RelationUpdated.Trigger(p, oldRelation)
}

// updates the alias of the given peer.
func (m *Manager) updateAlias(id peer.ID, alias string) {
	p, has := m.peers[id]
	if !has {
		return
	}
	p.Alias = alias
}

// schedules the reconnect timer for a reconnect attempt to the given peer,
// if the peer's relation is PeerRelationKnown.
func (m *Manager) scheduleReconnectIfKnown(id peer.ID) {
	p, has := m.peers[id]
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
		m.reconnectChan <- &reconnectmsg{id: id}
	})
	m.Events.ScheduledReconnect.Trigger(p, delay)
}

// resets the reconnect timer for the given peer.
func (m *Manager) resetReconnect(id peer.ID) {
	p, has := m.peers[id]
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
func (m *Manager) reconnectPeer(id peer.ID) (bool, error) {
	p, has := m.peers[id]
	if !has {
		return false, nil
	}

	if p.Relation != PeerRelationKnown || p.reconnectTimer == nil {
		return false, nil
	}

	m.Events.Reconnecting.Trigger(p)
	return true, m.connect(peer.AddrInfo{ID: id, Addrs: p.Addrs})
}

// connect does an actual connection attempt to the given peer.
// if the connection fails, the peer is either cleared from the Manager if its relation is PeerRelationUnknown
// or a reconnect attempt is scheduled if it is PeerRelationKnown.
func (m *Manager) connect(addrInfo peer.AddrInfo) error {
	ctx, _ := context.WithTimeout(context.Background(), connTimeout)
	err := m.host.Connect(ctx, addrInfo)
	if err != nil {
		// unsuccessful connect:
		// get rid of the peer instance if the relation is unknown
		// or initiate a reconnect timer
		m.cleanupPeerIfUnknown(addrInfo.ID)
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

// removes an unknown peer if it has no more connections.
func (m *Manager) cleanupPeerIfUnknown(id peer.ID) {
	p, has := m.peers[id]
	if has && p.Relation == PeerRelationUnknown &&
		len(m.host.Network().ConnsToPeer(id)) == 0 {
		delete(m.peers, id)
	}
}

// checks whether the given peer is connected.
func (m *Manager) isConnected(id peer.ID) bool {
	if _, has := m.peers[id]; !has {
		return false
	}
	return m.host.Network().Connectedness(id) == network.Connected
}

// calls the given PeerForEachFunc on each Peer.
func (m *Manager) forEach(f PeerForEachFunc, filter ...PeerRelation) {
	for _, p := range m.peers {
		if len(filter) > 0 {
			if p.Relation != filter[0] {
				continue
			}
		}
		if !f(p) {
			break
		}
	}
}

// calls the given PeerFunc if the given peer exists.
func (m *Manager) call(id peer.ID, f PeerFunc) {
	p, has := m.peers[id]
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
	m.Events.Disconnected.Attach(events.NewClosure(func(p *Peer) {
		m.opts.Logger.Infof("disconnected %s", p.ID.ShortString())
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
	m.connectedChan <- &connectionmsg{net: net, conn: conn}
}
func (m *netNotifiee) Disconnected(net network.Network, conn network.Conn) {
	m.disconnectedChan <- &connectionmsg{net: net, conn: conn}
}
func (m *netNotifiee) OpenedStream(net network.Network, stream network.Stream) {}
func (m *netNotifiee) ClosedStream(net network.Network, stream network.Stream) {}
