package processor

import (
	"github.com/gohornet/hornet/packages/peering/peer"
)

// Request defines a request from a peer for a given transaction or milestone.
type Request struct {
	// the peer from which this request is from.
	p *peer.Peer
	// the byte-encoded hash of the transaction which is requested.
	requestedTxHashBytes []byte
}

// Punish increments the peer's invalid received transactions metric.
func (r *Request) Punish() {
	r.p.Metrics.InvalidTransactions.Inc()
}

// Punish increments the peer's stale received transactions metric.
func (r *Request) Stale() {
	r.p.Metrics.StaleTransactions.Inc()
}
