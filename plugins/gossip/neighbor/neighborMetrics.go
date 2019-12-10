package neighbor

import (
	"sync/atomic"
)

// Defines the metrics of a Neighbor
type NeighborMetrics struct {
	allTxsCount               uint32
	invalidTxsCount           uint32
	staleTxsCount             uint32
	randomTxsCount            uint32
	sentTxsCount              uint32
	newTxsCount               uint32
	droppedSendPacketsCount   uint32
	receivedMilestoneReqCount uint32
	sentMilestoneReqCount     uint32
}

// Returns the number of all transactions.
func (nm *NeighborMetrics) GetAllTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.allTxsCount)
}

// Increments the all transactions count.
func (nm *NeighborMetrics) IncrAllTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.allTxsCount, 1)
}

// Gets the number of invalid transctions.
func (nm *NeighborMetrics) GetInvalidTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.invalidTxsCount)
}

// Increments the invalid transaction count.
func (nm *NeighborMetrics) IncrInvalidTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.invalidTxsCount, 1)
}

// Gets the number of stale transactions.
func (nm *NeighborMetrics) GetStaleTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.staleTxsCount)
}

// Gets the number of new transactions.
func (nm *NeighborMetrics) GetNewTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.newTxsCount)
}

// Increments the new transactions count.
func (nm *NeighborMetrics) IncrNewTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.newTxsCount, 1)
}

// Gets the number of random transactions.
func (nm *NeighborMetrics) GetRandomTransactionRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.randomTxsCount)
}

// Increments the random transactions count.
func (nm *NeighborMetrics) IncrRandomTransactionRequestsCount() uint32 {
	return atomic.AddUint32(&nm.randomTxsCount, 1)
}

// Gets the number of send transactions.
func (nm *NeighborMetrics) GetSentTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.sentTxsCount)
}

// Increments the send transactions count.
func (nm *NeighborMetrics) IncrSentTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.sentTxsCount, 1)
}

// Gets the number of packets dropped from the neighbor's send queue.
func (nm *NeighborMetrics) GetDroppedSendPacketsCount() uint32 {
	return atomic.LoadUint32(&nm.droppedSendPacketsCount)
}

// Increments the number of packets dropped from the neighbor's send queue.
func (nm *NeighborMetrics) IncrDroppedSendPacketsCount() uint32 {
	return atomic.AddUint32(&nm.droppedSendPacketsCount, 1)
}

// Gets the number of sent milestone requests.
func (nm *NeighborMetrics) GetSentMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.sentMilestoneReqCount)
}

// Increments the sent milestone requests count.
func (nm *NeighborMetrics) IncrSentMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&nm.sentMilestoneReqCount, 1)
}

// Gets the number of received milestone requests.
func (nm *NeighborMetrics) GetReceivedMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.receivedMilestoneReqCount)
}

// Increments the received milestone requests count.
func (nm *NeighborMetrics) IncrReceivedMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&nm.receivedMilestoneReqCount, 1)
}
