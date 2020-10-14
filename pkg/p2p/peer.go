package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// NewPeer creates a new Peer.
func NewPeer(id peer.ID, relation PeerRelation, addrs []multiaddr.Multiaddr, alias string) *Peer {
	return &Peer{
		ID:       id,
		Relation: relation,
		Addrs:    addrs,
		Alias:    alias,
	}
}

// Peer is a remote peer in the network.
type Peer struct {
	// The ID of the peer.
	ID peer.ID
	// The relation to the peer.
	Relation PeerRelation
	// The addresses under which the peer was added.
	Addrs []multiaddr.Multiaddr
	// The alias of the peer for better recognizing it.
	Alias string

	connectedEventCalled bool
	reconnectTimer       *time.Timer
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
	// The instance of the peer.
	Peer *Peer `json:"-"`
	// The ID of the peer.
	ID string `json:"address"`
	// The addresses of the peer.
	Addresses []multiaddr.Multiaddr `json:"addresses"`
	// The alias of the peer.
	Alias string `json:"alias,omitempty"`
	// The amount of sent packets to the peer.
	SentPackets uint32 `json:"sentPackets"`
	// The amount of dropped packets.
	DroppedSentPackets uint32 `json:"droppedSentPackets"`
	// Whether the peer is connected.
	Connected bool `json:"connected"`
}
