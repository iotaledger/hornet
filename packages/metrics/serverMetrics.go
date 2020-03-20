package metrics

import (
	"go.uber.org/atomic"
)

var (
	SharedServerMetrics = &ServerMetrics{}
)

// Defines the metrics of the server
type ServerMetrics struct {
	Transactions                atomic.Uint64
	NewTransactions             atomic.Uint64
	KnownTransactions           atomic.Uint64
	ConfirmedTransactions       atomic.Uint64
	InvalidTransactions         atomic.Uint64
	InvalidTransactionRequests  atomic.Uint64
	StaleTransactions           atomic.Uint64
	ReceivedTransactions        atomic.Uint64
	ReceivedMilestoneRequests   atomic.Uint64
	ReceivedTransactionRequests atomic.Uint64
	ReceivedHeartbeats          atomic.Uint64
	SentTransactions            atomic.Uint64
	SentTransactionRequests     atomic.Uint64
	SentMilestoneRequests       atomic.Uint64
	SentHeartbeats              atomic.Uint64
	DroppedMessages             atomic.Uint64
	SentSpamTransactions        atomic.Uint64
	ValidatedBundles            atomic.Uint64
	SeenSpentAddresses          atomic.Uint64
}
