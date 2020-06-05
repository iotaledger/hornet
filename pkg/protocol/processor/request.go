package processor

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/peering/peer"
)

// Request defines a request from a peer for a given transaction or milestone.
type Request struct {
	// the peer from which this request is from.
	p *peer.Peer
	// the hash of the transaction which is requested.
	requestedTxHash hornet.Hash
}

// Punish increments the peer's invalid received transactions metric.
func (r *Request) Punish() {
	r.p.Metrics.InvalidTransactions.Inc()
}

// Punish increments the peer's stale received transactions metric.
func (r *Request) Stale() {
	r.p.Metrics.StaleTransactions.Inc()
}

// Empty tells whether this request holds a request or simply acts
// as a way to determine that a transaction was received from a given peer.
func (r *Request) Empty() bool {
	return r.requestedTxHash == nil
}
