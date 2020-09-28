// snatched and "improved" from https://github.com/ipfs/go-ipfs/tree/master/peering
package p2p

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Seed the random number generator.
//
// We don't need good randomness, but we do need randomness.
const (
	// maxBackoff is the maximum time between reconnect attempts.
	maxBackoff = 10 * time.Minute
	// The backoff will be cut off when we get within 10% of the actual max.
	// If we go over the max, we'll adjust the delay down to a random value
	// between 90-100% of the max backoff.
	maxBackoffJitter = 10 // %
	connmgrTag       = "ipfs-peering"
	// This needs to be sufficient to prevent two sides from simultaneously
	// dialing.
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

// PeerHandler keeps track of additional metadata concerning a ID.
type PeerHandler struct {
	ID     peer.ID
	host   host.Host
	ctx    context.Context
	cancel context.CancelFunc

	ps *PeeringService

	mu             sync.Mutex
	addresses      []multiaddr.Multiaddr
	reconnectTimer *time.Timer

	nextDelay time.Duration
}

// updateAddrs sets the addresses for this ID.
func (ph *PeerHandler) updateAddrs(addrs []multiaddr.Multiaddr) {
	// Not strictly necessary, but it helps to not trust the calling code.
	addrCopy := make([]multiaddr.Multiaddr, len(addrs))
	copy(addrCopy, addrs)

	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.addresses = addrCopy
}

// addrs returns a shared slice of addresses for this ID. Do not modify.
func (ph *PeerHandler) addrs() []multiaddr.Multiaddr {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	return ph.addresses
}

// stop permanently stops the ID handler.
func (ph *PeerHandler) stop() {
	ph.cancel()

	ph.mu.Lock()
	defer ph.mu.Unlock()
	if ph.reconnectTimer != nil {
		ph.reconnectTimer.Stop()
		ph.reconnectTimer = nil
	}
}

func (ph *PeerHandler) nextBackoff() time.Duration {
	if ph.nextDelay < maxBackoff {
		ph.nextDelay += ph.nextDelay/2 + time.Duration(rand.Int63n(int64(ph.nextDelay)))
	}

	// If we've gone over the max backoff, reduce it under the max.
	if ph.nextDelay > maxBackoff {
		ph.nextDelay = maxBackoff
		// randomize the backoff a bit (10%).
		ph.nextDelay -= time.Duration(rand.Int63n(int64(maxBackoff) * maxBackoffJitter / 100))
	}

	return ph.nextDelay
}

func (ph *PeerHandler) reconnect() {
	// try connecting
	addrs := ph.addrs()

	ph.ps.Events.Reconnecting.Trigger(ph)

	err := ph.host.Connect(ph.ctx, peer.AddrInfo{ID: ph.ID, Addrs: addrs})
	if err != nil {
		ph.ps.Events.ReconnectFailed.Trigger(ph)
		// ok, we failed. Extend the timeout.
		ph.mu.Lock()
		if ph.reconnectTimer != nil {
			// only counts if the reconnectTimer still exists. If not, a
			// connection _was_ somehow established.
			ph.reconnectTimer.Reset(ph.nextBackoff())
		}
		// otherwise, someone else has stopped us so we can assume that
		// we're either connected or someone else will start us.
		ph.mu.Unlock()
	}

	// always call this
	// we could have connected since we processed the error
	ph.stopReconnectTimerIfConnected()
}

func (ph *PeerHandler) stopReconnectTimerIfConnected() {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if ph.reconnectTimer != nil && ph.host.Network().Connectedness(ph.ID) == network.Connected {
		ph.ps.Events.Reconnected.Trigger(ph)
		ph.reconnectTimer.Stop()
		ph.reconnectTimer = nil
		ph.ps.Events.Connected.Trigger(ph)
		ph.nextDelay = initialDelay
	}
}

// startReconnectTimerIfDisconnected is the inverse of stopReconnectTimerIfConnected.
func (ph *PeerHandler) startReconnectTimerIfDisconnected() {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if ph.reconnectTimer == nil && ph.host.Network().Connectedness(ph.ID) != network.Connected {
		ph.ps.Events.Disconnected.Trigger(ph)
		// always start with a short timeout so we can stagger things a bit.
		ph.reconnectTimer = time.AfterFunc(ph.nextBackoff(), ph.reconnect)
	}
}

// PeerHandlerCaller is an event handler called from PeerHandler.
func PeerHandlerCaller(handler interface{}, params ...interface{}) {
	handler.(func(peerHandler *PeerHandler))(params[0].(*PeerHandler))
}

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
	host host.Host

	mu    sync.RWMutex
	peers map[peer.ID]*PeerHandler
	state state

	Events PeeringServiceEvents
}

// NewPeeringService constructs a new peering service. Peers can be added and
// removed immediately, but connections won't be formed until PeeringService.Start() is called.
func NewPeeringService(host host.Host) *PeeringService {
	return &PeeringService{
		host:  host,
		peers: make(map[peer.ID]*PeerHandler),
		Events: PeeringServiceEvents{
			Reconnecting:    events.NewEvent(PeerHandlerCaller),
			ReconnectFailed: events.NewEvent(PeerHandlerCaller),
			Reconnected:     events.NewEvent(PeerHandlerCaller),
			Connected:       events.NewEvent(PeerHandlerCaller),
			Disconnected:    events.NewEvent(PeerHandlerCaller),
			UpdatedAddrs:    events.NewEvent(PeerHandlerCaller),
			Added:           events.NewEvent(PeerHandlerCaller),
			Removed:         events.NewEvent(PeerHandlerCaller),
			ServiceStarted:  events.NewEvent(events.CallbackCaller),
			ServiceStopped:  events.NewEvent(events.CallbackCaller),
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
	// just update the addresses
	if ok {
		ps.Events.UpdatedAddrs.Trigger(handler)
		handler.updateAddrs(info.Addrs)
		return
	}

	ps.host.ConnManager().Protect(info.ID, connmgrTag)
	handler = &PeerHandler{
		host:      ps.host,
		ID:        info.ID,
		addresses: info.Addrs,
		nextDelay: initialDelay,
		ps:        ps,
	}

	ps.Events.Added.Trigger(handler)

	handler.ctx, handler.cancel = context.WithCancel(context.Background())
	ps.peers[info.ID] = handler
	switch ps.state {
	case stateRunning:
		go handler.startReconnectTimerIfDisconnected()
	case stateStopped:
		// we still construct everything in this state because
		// it's easier to reason about. but we should still free
		// resources.
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

// implements network.Notifiee for PeeringService
type netNotifee PeeringService

func (nn *netNotifee) Connected(_ network.Network, c network.Conn) {
	ps := (*PeeringService)(nn)

	p := c.RemotePeer()
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if handler, ok := ps.peers[p]; ok {
		// use a goroutine to avoid blocking events.
		go handler.stopReconnectTimerIfConnected()
	}
}
func (nn *netNotifee) Disconnected(_ network.Network, c network.Conn) {
	ps := (*PeeringService)(nn)

	p := c.RemotePeer()
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if handler, ok := ps.peers[p]; ok {
		// use a goroutine to avoid blocking events.
		go handler.startReconnectTimerIfDisconnected()
	}
}
func (nn *netNotifee) OpenedStream(network.Network, network.Stream)     {}
func (nn *netNotifee) ClosedStream(network.Network, network.Stream)     {}
func (nn *netNotifee) Listen(network.Network, multiaddr.Multiaddr)      {}
func (nn *netNotifee) ListenClose(network.Network, multiaddr.Multiaddr) {}
