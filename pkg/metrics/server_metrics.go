package metrics

import (
	"go.uber.org/atomic"
)

// ServerMetrics defines metrics over the entire runtime of the node.
type ServerMetrics struct {
	// The number of total received blocks.
	Blocks atomic.Uint32
	// The number of received blocks which are new.
	NewBlocks atomic.Uint32
	// The number of received blocks which are already known.
	KnownBlocks atomic.Uint32
	// The number of referenced blocks.
	ReferencedBlocks atomic.Uint32
	// The number of blocks with a transaction payload.
	IncludedTransactionBlocks atomic.Uint32
	// The number of blocks without a transaction payload.
	NoTransactionBlocks atomic.Uint32
	// The number of blocks with conflicting transaction payloads.
	ConflictingTransactionBlocks atomic.Uint32
	// The number of received invalid blocks.
	InvalidBlocks atomic.Uint32
	// The number of received invalid requests (both blocks and milestones).
	InvalidRequests atomic.Uint32
	// The number of received milestone requests.
	ReceivedMilestoneRequests atomic.Uint32
	// The number of received block requests.
	ReceivedBlockRequests atomic.Uint32
	// The number of received heartbeats.
	ReceivedHeartbeats atomic.Uint32
	// The number of sent blocks.
	SentBlocks atomic.Uint32
	// The number of sent block requests.
	SentBlockRequests atomic.Uint32
	// The number of sent milestone requests.
	SentMilestoneRequests atomic.Uint32
	// The number of sent heartbeats.
	SentHeartbeats atomic.Uint32
	// The number of dropped packets.
	DroppedPackets atomic.Uint32
	// The number of sent spam blocks.
	SentSpamBlocks atomic.Uint32
	// The number of non-lazy tips.
	TipsNonLazy atomic.Uint32
	// The number of semi-lazy tips.
	TipsSemiLazy atomic.Uint32
}
