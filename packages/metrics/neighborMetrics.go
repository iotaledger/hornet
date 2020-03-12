package metrics

import (
	"sync/atomic"
)

// Defines the metrics of a Neighbor
type NeighborMetrics struct {
	// Receive
	allTxsCount               uint32
	newTxsCount               uint32
	knownTxsCount             uint32
	invalidTxsCount           uint32
	invalidRequestsCount      uint32
	staleTxsCount             uint32
	receivedTxReqCount        uint32
	receivedMilestoneReqCount uint32
	receivedHeartbeatsCount   uint32

	// Transmit
	sentTxsCount            uint32
	sentTxsReqCount         uint32
	sentMilestoneReqCount   uint32
	sentHeartbeatsCount     uint32
	droppedSendPacketsCount uint32
}

//////////////////// Receive ////////////////////

// Returns the number of all transactions.
func (nm *NeighborMetrics) GetAllTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.allTxsCount)
}

// Increments the all transactions count.
func (nm *NeighborMetrics) IncrAllTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.allTxsCount, 1)
}

// Gets the number of new transactions.
func (nm *NeighborMetrics) GetNewTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.newTxsCount)
}

// Increments the new transactions count.
func (nm *NeighborMetrics) IncrNewTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.newTxsCount, 1)
}

// Gets the number of known transactions.
func (nm *NeighborMetrics) GetKnownTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.knownTxsCount)
}

// Increments the known transactions count.
func (nm *NeighborMetrics) IncrKnownTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.knownTxsCount, 1)
}

// Gets the number of invalid transactions.
func (nm *NeighborMetrics) GetInvalidTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.invalidTxsCount)
}

// Increments the invalid transaction count.
func (nm *NeighborMetrics) IncrInvalidTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.invalidTxsCount, 1)
}

// Gets the number of requests transactions.
func (nm *NeighborMetrics) GetInvalidRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.invalidRequestsCount)
}

// Increments the invalid requests count.
func (nm *NeighborMetrics) IncrInvalidRequestsCount() uint32 {
	return atomic.AddUint32(&nm.invalidRequestsCount, 1)
}

// Gets the number of stale transactions.
func (nm *NeighborMetrics) GetStaleTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.staleTxsCount)
}

// Increments the number of stale transactions.
func (nm *NeighborMetrics) IncrStaleTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.staleTxsCount, 1)
}

// Gets the number of received transaction requests.
func (nm *NeighborMetrics) GetReceivedTransactionRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.receivedTxReqCount)
}

// Increments the received transaction requests count.
func (nm *NeighborMetrics) IncrReceivedTransactionRequestsCount() uint32 {
	return atomic.AddUint32(&nm.receivedTxReqCount, 1)
}

// Gets the number of received milestone requests.
func (nm *NeighborMetrics) GetReceivedMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.receivedMilestoneReqCount)
}

// Increments the received milestone requests count.
func (nm *NeighborMetrics) IncrReceivedMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&nm.receivedMilestoneReqCount, 1)
}

// Gets the number of received heartbeats.
func (nm *NeighborMetrics) GetReceivedHeartbeatsCount() uint32 {
	return atomic.LoadUint32(&nm.receivedHeartbeatsCount)
}

// Increments the received heartbeats count.
func (nm *NeighborMetrics) IncrReceivedHeartbeatsCount() uint32 {
	return atomic.AddUint32(&nm.receivedHeartbeatsCount, 1)
}

//////////////////// Transmit ////////////////////

// Gets the number of send transactions.
func (nm *NeighborMetrics) GetSentTransactionsCount() uint32 {
	return atomic.LoadUint32(&nm.sentTxsCount)
}

// Increments the send transactions count.
func (nm *NeighborMetrics) IncrSentTransactionsCount() uint32 {
	return atomic.AddUint32(&nm.sentTxsCount, 1)
}

// Gets the number of send transaction requests count.
func (nm *NeighborMetrics) GetSentTransactionRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.sentTxsReqCount)
}

// Increments the send transactions requests count.
func (nm *NeighborMetrics) IncrSentTransactionRequestsCount() uint32 {
	return atomic.AddUint32(&nm.sentTxsReqCount, 1)
}

// Gets the number of sent milestone requests.
func (nm *NeighborMetrics) GetSentMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&nm.sentMilestoneReqCount)
}

// Increments the sent milestone requests count.
func (nm *NeighborMetrics) IncrSentMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&nm.sentMilestoneReqCount, 1)
}

// Gets the number of sent heartbeats.
func (nm *NeighborMetrics) GetSentHeartbeatsCount() uint32 {
	return atomic.LoadUint32(&nm.sentHeartbeatsCount)
}

// Increments the sent heartbeats count.
func (nm *NeighborMetrics) IncrSentHeartbeatsCount() uint32 {
	return atomic.AddUint32(&nm.sentHeartbeatsCount, 1)
}

// Gets the number of packets dropped from the neighbor's send queue.
func (nm *NeighborMetrics) GetDroppedSendPacketsCount() uint32 {
	return atomic.LoadUint32(&nm.droppedSendPacketsCount)
}

// Increments the number of packets dropped from the neighbor's send queue.
func (nm *NeighborMetrics) IncrDroppedSendPacketsCount() uint32 {
	return atomic.AddUint32(&nm.droppedSendPacketsCount, 1)
}
