package metrics

import (
	"go.uber.org/atomic"
)

// ServerMetrics defines metrics over the entire runtime of the node.
type ServerMetrics struct {
	// The number of total received messages.
	Messages atomic.Uint32
	// The number of received messages which are new.
	NewMessages atomic.Uint32
	// The number of received messages which are already known.
	KnownMessages atomic.Uint32
	// The number of referenced messages.
	ReferencedMessages atomic.Uint32
	// The number of messages with a transaction payload.
	IncludedTransactionMessages atomic.Uint32
	// The number of messages without a transaction payload.
	NoTransactionMessages atomic.Uint32
	// The number of messages with conflicting transaction payloads.
	ConflictingTransactionMessages atomic.Uint32
	// The number of received invalid messages.
	InvalidMessages atomic.Uint32
	// The number of received invalid requests (both messages and milestones).
	InvalidRequests atomic.Uint32
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint32
	// The number of received message requests.
	ReceivedMessageRequests atomic.Uint32
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint32
	// The number of sent messages.
	SentMessages atomic.Uint32
	// The number of sent message requests.
	SentMessageRequests atomic.Uint32
	// The number of sent milestone requests.
	SentMilestoneRequests atomic.Uint32
	// The number of sent heartbeats.
	SentHeartbeats atomic.Uint32
	// The number of dropped messages.
	DroppedMessages atomic.Uint32
	// The number of sent spam messages.
	SentSpamMessages atomic.Uint32
	// The number of validated messages.
	ValidatedMessages atomic.Uint32
	// The number of non-lazy tips.
	TipsNonLazy atomic.Uint32
	// The number of semi-lazy tips.
	TipsSemiLazy atomic.Uint32
}
