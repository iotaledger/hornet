package p2p

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// PeerEvents are events fired by the Peer.
type PeerEvents struct {
	// Fired when the peer got disconnected.
	Disconnected *events.Event
}

// Peer keeps track of additional metadata concerning a peer.
type Peer struct {
	// The peer's ID.
	ID peer.ID

	// Events happening on the peer.
	Events PeerEvents

	host   host.Host
	ctx    context.Context
	cancel context.CancelFunc

	firstConnect bool

	ps *PeeringService

	mu             sync.Mutex
	addresses      []multiaddr.Multiaddr
	reconnectTimer *time.Timer

	nextDelay time.Duration
}

// Info returns a snapshot of the peer in time of calling Info().
func (p *Peer) Info() *PeerSnapshot {
	info := &PeerSnapshot{
		Peer:      p,
		ID:        p.ID.String(),
		Addresses: p.addresses,
	}
	return info
}

// Disconnect disconnects the peer's connections and initiates the reconnect timer.
func (p *Peer) Disconnect() {
	p.cancel()
	p.ctx, p.cancel = context.WithCancel(context.Background())
}

// IsConnected tells whether the peer is currently connected.
func (p *Peer) IsConnected(host host.Host) bool {
	return host.Network().Connectedness(p.ID) == network.Connected
}

// updateAddrs sets the addresses for this Peer.
func (p *Peer) updateAddrs(addrs []multiaddr.Multiaddr) {
	// Not strictly necessary, but it helps to not trust the calling code.
	addrCopy := make([]multiaddr.Multiaddr, len(addrs))
	copy(addrCopy, addrs)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.addresses = addrCopy
}

// addrs returns a shared slice of addresses for this Peer. Do not modify.
func (p *Peer) addrs() []multiaddr.Multiaddr {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addresses
}

// stop permanently stops the Peer.
func (p *Peer) stop() {
	p.cancel()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reconnectTimer != nil {
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil
		return
	}
	p.Events.Disconnected.Trigger()
}

func (p *Peer) nextBackoff() time.Duration {
	if p.nextDelay < maxBackoff {
		p.nextDelay += p.nextDelay/2 + time.Duration(rand.Int63n(int64(p.nextDelay)))
	}

	// If we've gone over the max backoff, reduce it under the max.
	if p.nextDelay > maxBackoff {
		p.nextDelay = maxBackoff
		// randomize the backoff a bit (10%).
		p.nextDelay -= time.Duration(rand.Int63n(int64(maxBackoff) * maxBackoffJitter / 100))
	}

	return p.nextDelay
}

func (p *Peer) reconnect() {
	// try connecting
	addrs := p.addrs()

	if !p.firstConnect {
		p.ps.Events.Reconnecting.Trigger(p)
	}

	err := p.host.Connect(p.ctx, peer.AddrInfo{ID: p.ID, Addrs: addrs})
	if err != nil {
		p.ps.Events.ReconnectFailed.Trigger(p)
		// ok, we failed. Extend the timeout.
		p.mu.Lock()
		if p.reconnectTimer != nil {
			// only counts if the reconnectTimer still exists. If not, a
			// connection _was_ somehow established.
			p.reconnectTimer.Reset(p.nextBackoff())
		}
		// otherwise, someone else has stopped us so we can assume that
		// we're either connected or someone else will start us.
		p.mu.Unlock()
	}

	// always call this
	// we could have connected since we processed the error
	p.stopReconnectTimerIfConnected()
}

func (p *Peer) stopReconnectTimerIfConnected() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.firstConnect = false
	if p.reconnectTimer != nil && p.host.Network().Connectedness(p.ID) == network.Connected {
		p.ps.Events.Reconnected.Trigger(p)
		p.reconnectTimer.Stop()
		p.reconnectTimer = nil
		p.ps.Events.Connected.Trigger(p)
		p.nextDelay = initialDelay
	}
}

// startReconnectTimerIfDisconnected is the inverse of stopReconnectTimerIfConnected.
func (p *Peer) startReconnectTimerIfDisconnected() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.reconnectTimer == nil && p.host.Network().Connectedness(p.ID) != network.Connected {
		p.ps.Events.Disconnected.Trigger(p)
		p.Events.Disconnected.Trigger()
		// always start with a short timeout so we can stagger things a bit.
		p.reconnectTimer = time.AfterFunc(p.nextBackoff(), p.reconnect)
	}
}

// PeerHandlerCaller is an event handler called with a Peer.
func PeerHandlerCaller(handler interface{}, params ...interface{}) {
	handler.(func(peerHandler *Peer))(params[0].(*Peer))
}

// PeerIDCaller is an event handler called with a peer.ID.
func PeerIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(peerID peer.ID))(params[0].(peer.ID))
}

// PeerSnapshot acts as a static snapshot of information about a peer.
type PeerSnapshot struct {
	Peer               *Peer                 `json:"-"`
	ID                 string                `json:"address"`
	Addresses          []multiaddr.Multiaddr `json:"addresses"`
	Alias              string                `json:"alias,omitempty"`
	PreferIPv6         bool                  `json:"-"`
	SentPackets        uint32                `json:"sentPackets"`
	DroppedSentPackets uint32                `json:"droppedSentPackets"`
	Connected          bool                  `json:"connected"`
}
