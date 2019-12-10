package server

import (
	"sync/atomic"
)

var (
	SharedServerMetrics = &ServerMetrics{}
)

// Defines the metrics of the server
type ServerMetrics struct {
	allTxsCount               uint32
	invalidTxsCount           uint32
	staleTxsCount             uint32
	randomTxsCount            uint32
	sentTxsCount              uint32
	receivedMilestoneReqCount uint32
	sentMilestoneReqCount     uint32
	newTxsCount               uint32
	droppedSendPacketsCount   uint32
	receivedTxReqCount        uint32
	sentTxReqCount            uint32
}

// Returns the number of all transactions.
func (sm *ServerMetrics) GetAllTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.allTxsCount)
}

// Increments the all transactions count.
func (sm *ServerMetrics) IncrAllTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.allTxsCount, 1)
}

// Gets the number of invalid transactions.
func (sm *ServerMetrics) GetInvalidTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.invalidTxsCount)
}

// Increments the invalid transaction count.
func (sm *ServerMetrics) IncrInvalidTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.invalidTxsCount, 1)
}

// Gets the number of stale transactions.
func (sm *ServerMetrics) GetStaleTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.staleTxsCount)
}

// Gets the number of new transactions.
func (sm *ServerMetrics) GetNewTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.newTxsCount)
}

// Increments the new transactions count.
func (sm *ServerMetrics) IncrNewTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.newTxsCount, 1)
}

// Gets the number of random transactions.
func (sm *ServerMetrics) GetRandomTransactionRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.randomTxsCount)
}

// Increments the random transactions count.
func (sm *ServerMetrics) IncrRandomTransactionRequestsCount() uint32 {
	return atomic.AddUint32(&sm.randomTxsCount, 1)
}

// Gets the number of send transactions.
func (sm *ServerMetrics) GetSentTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.sentTxsCount)
}

// Increments the send transactions count.
func (sm *ServerMetrics) IncrSentTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.sentTxsCount, 1)
}

// Gets the number of send transaction requests.
func (sm *ServerMetrics) GetSentTransactionRequestCount() uint32 {
	return atomic.LoadUint32(&sm.sentTxReqCount)
}

// Increments the send transaction request count.
func (sm *ServerMetrics) IncrSentTransactionRequestCount() uint32 {
	return atomic.AddUint32(&sm.sentTxReqCount, 1)
}

// Gets the number of received milestone requests.
func (sm *ServerMetrics) GetReceivedMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.receivedMilestoneReqCount)
}

// Increments the received milestone requests count.
func (sm *ServerMetrics) IncrReceivedMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&sm.receivedMilestoneReqCount, 1)
}

// Gets the number of send milestone requests.
func (sm *ServerMetrics) GetSentMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.sentMilestoneReqCount)
}

// Increments the send milestone requests count.
func (sm *ServerMetrics) IncrSentMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&sm.sentMilestoneReqCount, 1)
}

// Gets the number of received transactions requests count.
func (sm *ServerMetrics) GetReceivedTransactionRequestCount() uint32 {
	return atomic.LoadUint32(&sm.receivedTxReqCount)
}

// Increments the received transactions requests count.
func (sm *ServerMetrics) IncrReceivedTransactionRequestCount() uint32 {
	return atomic.AddUint32(&sm.receivedTxReqCount, 1)
}

// Gets the number of packets dropped from the send queue.
func (sm *ServerMetrics) GetDroppedSendPacketsCount() uint32 {
	return atomic.LoadUint32(&sm.droppedSendPacketsCount)
}

// Increments the number of packets dropped from the send queue.
func (sm *ServerMetrics) IncrDroppedSendPacketsCount() uint32 {
	return atomic.AddUint32(&sm.droppedSendPacketsCount, 1)
}
