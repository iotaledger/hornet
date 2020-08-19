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
	Transactions atomic.Uint32
	// The number of received transactions which are new.
	NewTransactions atomic.Uint32
	// The number of received transactions which are already known.
	KnownTransactions atomic.Uint32
	// The number of confirmed transactions.
	ConfirmedTransactions atomic.Uint32
	// The number of value transactions.
	ValueTransactions atomic.Uint32
	// The number of zero value transactions.
	ZeroValueTransactions atomic.Uint32
	// The number of conflicting transactions.
	ConflictingTransactions atomic.Uint32
	// The number of received invalid transactions.
	InvalidTransactions atomic.Uint32
	// The number of received invalid requests (both transactions and milestones).
	InvalidRequests atomic.Uint32
	// The number of received transactions of which their timestamp is stale.
	StaleTransactions atomic.Uint32
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint32
	// The number of received transaction requests.
	ReceivedTransactionRequests atomic.Uint32
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint32
	// The number of sent transactions.
	SentTransactions atomic.Uint32
	// The number of sent transaction requests.
	SentTransactionRequests atomic.Uint32
	// The number of sent milestone requests.
	SentMilestoneRequests atomic.Uint32
	// The number of sent heartbeats.
	SentHeartbeats atomic.Uint32
	// The number of dropped messages.
	DroppedMessages atomic.Uint32
	// The number of sent spam transactions.
	SentSpamTransactions atomic.Uint32
	// The number of validated bundles.
	ValidatedBundles atomic.Uint32
	// The number of seen spent addresses.
	SeenSpentAddresses atomic.Uint32
	// The number of non-lazy tips.
	TipsNonLazy atomic.Uint32
	// The number of semi-lazy tips.
	TipsSemiLazy atomic.Uint32
}
