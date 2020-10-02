// snatched and "improved" from https://github.com/ipfs/go-ipfs/tree/master/peering
package p2p

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// TODO: seed the random number generator.
// we don't need good randomness, but we do need randomness.
const (
	// maxBackoff is the maximum time between reconnect attempts.
	maxBackoff = 10 * time.Minute

	// the backoff will be cut off when we get within 10% of the actual max.
	// if we go over the max, we'll adjust the delay down to a random value
	// between 90-100% of the max backoff.
	maxBackoffJitter = 10 // %
	connmgrTag       = "hornet-peering"

	// this needs to be sufficient to prevent two sides from simultaneously dialing.
	initialDelay = 5 * time.Second
)

var (
	// Returned when the PeeringService is tried to be shut down while it already is.
	ErrPeeringServiceAlreadyStopped = errors.New("peering service is already stopped")
)

// the state of the PeeringService.
type state int

const (
	stateInit state = iota
	stateRunning
	stateStopped
)

// PeeringServiceEvents are events fired by the PeeringService.
type PeeringServiceEvents struct {
	// Fired when a reconnect attempt is started.
	Reconnecting *events.Event
	// Fired when a reconnect attempt failed.
	ReconnectFailed *events.Event
	// Fired when a reconnect attempt succeeded.
	Reconnected *events.Event
	// Fired when a peer disconnected.
	Disconnected *events.Event
	// Fired when a connection to an unknown peer is closed.
	ClosedConnectionToUnknownPeer *events.Event
	// Fired when a peer has been connected.
	Connected *events.Event
	// Fired when a peer's addresses have been updated.
	UpdatedAddrs *events.Event
	// Fired when a peer has been added.
	Added *events.Event
	// Fired when a peer has been removed.
	Removed *events.Event
	// Fired when the service is started.
	ServiceStarted *events.Event
	// Fired when the service is stopped.
	ServiceStopped *events.Event
}

// PeeringService maintains connections to specified peers, reconnecting on
// disconnect with a back-off, while holding additional metadata about the peers.
type PeeringService struct {
	Events PeeringServiceEvents

	host host.Host

	mu    sync.RWMutex
	peers map[peer.ID]*Peer
	state state
}

// NewPeeringService constructs a new peering service. Peers can be added and
// removed immediately, but connections won't be formed until PeeringService.Start() is called.
func NewPeeringService(host host.Host) *PeeringService {
	return &PeeringService{
		host:  host,
		peers: make(map[peer.ID]*Peer),
		Events: PeeringServiceEvents{
			Reconnecting:                  events.NewEvent(PeerHandlerCaller),
			ReconnectFailed:               events.NewEvent(PeerHandlerCaller),
			Reconnected:                   events.NewEvent(PeerHandlerCaller),
			Connected:                     events.NewEvent(PeerHandlerCaller),
			Disconnected:                  events.NewEvent(PeerHandlerCaller),
			ClosedConnectionToUnknownPeer: events.NewEvent(PeerIDCaller),
			UpdatedAddrs:                  events.NewEvent(PeerHandlerCaller),
			Added:                         events.NewEvent(PeerHandlerCaller),
			Removed:                       events.NewEvent(PeerHandlerCaller),
			ServiceStarted:                events.NewEvent(events.CallbackCaller),
			ServiceStopped:                events.NewEvent(events.CallbackCaller),
		},
	}
}

// Start starts the peering service, connecting and maintaining connections to
// all registered peers. It returns an error if the service has already been
// stopped.
func (ps *PeeringService) Start() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	switch ps.state {
	case stateInit:
		ps.Events.ServiceStarted.Trigger()
	case stateRunning:
		return nil
	case stateStopped:
		return ErrPeeringServiceAlreadyStopped
	}

	ps.host.Network().Notify((*netNotifee)(ps))
	ps.state = stateRunning
	for _, handler := range ps.peers {
		go handler.startReconnectTimerIfDisconnected()
	}

	return nil
}

// Stop stops the peering service.
func (ps *PeeringService) Stop() error {
	ps.host.Network().StopNotify((*netNotifee)(ps))

	ps.mu.Lock()
	defer ps.mu.Unlock()

	switch ps.state {
	case stateInit, stateRunning:
		ps.Events.ServiceStopped.Trigger()
		for _, handler := range ps.peers {
			handler.stop()
		}
		ps.state = stateStopped
	}
	return nil
}

// Peer gets the peer by the given ID or nil.
func (ps *PeeringService) Peer(id peer.ID) *Peer {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.peers[id]
}

// AddPeer adds a ID to the peering service. This function may be safely
// called at any time: before the service is started, while running, or after it
// stops.
//
// Add ID may also be called multiple times for the same ID. The new
// addresses will replace the old.
func (ps *PeeringService) AddPeer(info peer.AddrInfo) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	handler, ok := ps.peers[info.ID]
	if ok {
		// just update the addresses
		ps.Events.UpdatedAddrs.Trigger(handler)
		handler.updateAddrs(info.Addrs)
		return
	}

	ps.host.ConnManager().Protect(info.ID, connmgrTag)
	handler = &Peer{
		ID: info.ID,
		Events: PeerEvents{
			Disconnected: events.NewEvent(events.CallbackCaller),
		},
		host:         ps.host,
		firstConnect: true,
		ps:           ps,
		mu:           sync.Mutex{},
		addresses:    info.Addrs,
		nextDelay:    initialDelay,
	}

	ps.Events.Added.Trigger(handler)

	handler.ctx, handler.cancel = context.WithCancel(context.Background())
	ps.peers[info.ID] = handler
	switch ps.state {
	case stateRunning:
		go handler.startReconnectTimerIfDisconnected()
	case stateStopped:
		// we still construct everything in this state because
		// it's easier to reason about. but we should still free resources.
		handler.cancel()
	}

}

// RemovePeer removes a ID from the peering service. This function may be
// safely called at any time: before the service is started, while running, or
// after it stops.
func (ps *PeeringService) RemovePeer(id peer.ID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if handler, ok := ps.peers[id]; ok {
		ps.Events.Removed.Trigger(handler)
		ps.host.ConnManager().Unprotect(id, connmgrTag)

		handler.stop()
		delete(ps.peers, id)
	}
}

// PeerConsumer is a function which takes a Peer and tells
// whether further iteration is needed by returning true.
type PeerConsumer func(p *Peer) bool

// ForAllConnected calls the given PeerConsumer on all peers.
func (ps *PeeringService) ForAllConnected(peerConsumer PeerConsumer) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, p := range ps.peers {
		if !p.IsConnected(ps.host) {
			continue
		}
		if con := peerConsumer(p); !con {
			break
		}
	}
}

// PeerCount returns the current count of peers.
func (ps *PeeringService) PeerCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.peers)
}

// ConnectedPeerCount returns the current count of connected peers.
func (ps *PeeringService) ConnectedPeerCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var connected int
	for _, p := range ps.peers {
		if p.IsConnected(ps.host) {
			connected++
		}
	}
	return connected
}

// PeerSnapshots returns snapshots of information of the currently connected/to-be-reconnected peers.
func (ps *PeeringService) PeerSnapshots() []*PeerSnapshot {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	infos := make([]*PeerSnapshot, 0)
	for _, p := range ps.peers {
		info := p.Info()
		info.Connected = p.IsConnected(ps.host)
		infos = append(infos, info)
	}
	return infos
}

// implements network.Notifiee for PeeringService.
type netNotifee PeeringService

func (nn *netNotifee) Connected(_ network.Network, c network.Conn) {
	ps := (*PeeringService)(nn)

	p := c.RemotePeer()
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if handler, ok := ps.peers[p]; ok {
		// use a goroutine to avoid blocking events.
		go handler.stopReconnectTimerIfConnected()
		return
	}

	// close connections to peers we don't know
	_ = c.Close()
	nn.Events.ClosedConnectionToUnknownPeer.Trigger(p)

}
func (nn *netNotifee) Disconnected(_ network.Network, c network.Conn) {
	ps := (*PeeringService)(nn)

	remotePeerID := c.RemotePeer()
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if p, ok := ps.peers[remotePeerID]; ok {
		// use a goroutine to avoid blocking events.
		go p.startReconnectTimerIfDisconnected()
		return
	}
}
func (nn *netNotifee) OpenedStream(network.Network, network.Stream)     {}
func (nn *netNotifee) ClosedStream(network.Network, network.Stream)     {}
func (nn *netNotifee) Listen(network.Network, multiaddr.Multiaddr)      {}
func (nn *netNotifee) ListenClose(network.Network, multiaddr.Multiaddr) {}
