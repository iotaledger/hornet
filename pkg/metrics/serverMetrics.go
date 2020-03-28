package metrics

import (
	"go.uber.org/atomic"
)

var (
	SharedServerMetrics = &ServerMetrics{}
)

// ServerMetrics defines metrics over the entire runtime of the node.
type ServerMetrics struct {
	// The number of total received transactions.
	Transactions atomic.Uint64
	// The number of received transactions which are new.
	NewTransactions atomic.Uint64
	// The number of received transactions which are already known.
	KnownTransactions atomic.Uint64
	// The number of confirmed transactions.
	ConfirmedTransactions atomic.Uint64
	// The number of received invalid transactions.
	InvalidTransactions atomic.Uint64
	// The number of received invalid requests (both transactions and milestones).
	InvalidRequests atomic.Uint64
	// The number of received transactions of which their timestamp is stale.
	StaleTransactions atomic.Uint64
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint64
	// The number of received transaction requests.
	ReceivedTransactionRequests atomic.Uint64
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint64
	// The number of sent transactions.
	SentTransactions atomic.Uint64
	// The number of sent transaction requests.
	SentTransactionRequests atomic.Uint64
	// The number of sent milestone requests.
	SentMilestoneRequests atomic.Uint64
	// The number of sent heartbeats.
	SentHeartbeats atomic.Uint64
	// The number of dropped messages.
	DroppedMessages atomic.Uint64
	// The number of sent spam transactions.
	SentSpamTransactions atomic.Uint64
	// The number of validated bundles.
	ValidatedBundles atomic.Uint64
	// The number of seen spent addresses.
	SeenSpentAddresses atomic.Uint64
}
