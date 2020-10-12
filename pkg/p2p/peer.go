package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// NewPeer creates a new Peer.
func NewPeer(id peer.ID, relation PeerRelation, addrs []multiaddr.Multiaddr) *Peer {
	return &Peer{
		ID:       id,
		Relation: relation,
		Addrs:    addrs,
	}
}

// Peer is a remote peer in the network.
type Peer struct {
	ID       peer.ID
	Conn     network.Conn
	Relation PeerRelation
	Addrs    []multiaddr.Multiaddr

	connectedEventCalled bool
	reconnectTimer       *time.Timer
	nextReconnectDelay   time.Duration
}

// Info returns a snapshot of the peer in time of calling Info().
func (p *Peer) Info() *PeerSnapshot {
	info := &PeerSnapshot{
		Peer:      p,
		ID:        p.ID.String(),
		Addresses: p.Addrs,
	}
	return info
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
