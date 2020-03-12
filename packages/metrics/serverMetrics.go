package metrics

import (
	"sync/atomic"
)

var (
	SharedServerMetrics = &ServerMetrics{}
)

// Defines the metrics of the server
type ServerMetrics struct {
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

	// Spammer
	sentSpamTxsCount uint32

	// Global
	validatedBundlesCount uint32
	seenSpentAddrCount    uint32
}

//////////////////// Receive ////////////////////

// Returns the number of all transactions.
func (sm *ServerMetrics) GetAllTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.allTxsCount)
}

// Increments the all transactions count.
func (sm *ServerMetrics) IncrAllTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.allTxsCount, 1)
}

// Gets the number of new transactions.
func (sm *ServerMetrics) GetNewTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.newTxsCount)
}

// Increments the new transactions count.
func (sm *ServerMetrics) IncrNewTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.newTxsCount, 1)
}

// Gets the number of known transactions.
func (sm *ServerMetrics) GetKnownTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.knownTxsCount)
}

// Increments the known transactions count.
func (sm *ServerMetrics) IncrKnownTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.knownTxsCount, 1)
}

// Gets the number of invalid transactions.
func (sm *ServerMetrics) GetInvalidTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.invalidTxsCount)
}

// Increments the invalid transaction count.
func (sm *ServerMetrics) IncrInvalidTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.invalidTxsCount, 1)
}

// Gets the number of requests transactions.
func (sm *ServerMetrics) GetInvalidRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.invalidRequestsCount)
}

// Increments the invalid requests count.
func (sm *ServerMetrics) IncrInvalidRequestsCount() uint32 {
	return atomic.AddUint32(&sm.invalidRequestsCount, 1)
}

// Gets the number of stale transactions.
func (sm *ServerMetrics) GetStaleTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.staleTxsCount)
}

// Increments the number of stale transactions.
func (sm *ServerMetrics) IncrStaleTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.staleTxsCount, 1)
}

// Gets the number of received transaction requests.
func (sm *ServerMetrics) GetReceivedTransactionRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.receivedTxReqCount)
}

// Increments the received transaction requests count.
func (sm *ServerMetrics) IncrReceivedTransactionRequestsCount() uint32 {
	return atomic.AddUint32(&sm.receivedTxReqCount, 1)
}

// Gets the number of received milestone requests.
func (sm *ServerMetrics) GetReceivedMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.receivedMilestoneReqCount)
}

// Increments the received milestone requests count.
func (sm *ServerMetrics) IncrReceivedMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&sm.receivedMilestoneReqCount, 1)
}

// Gets the number of received heartbeats.
func (sm *ServerMetrics) GetReceivedHeartbeatsCount() uint32 {
	return atomic.LoadUint32(&sm.receivedHeartbeatsCount)
}

// Increments the received heartbeats count.
func (sm *ServerMetrics) IncrReceivedHeartbeatsCount() uint32 {
	return atomic.AddUint32(&sm.receivedHeartbeatsCount, 1)
}

//////////////////// Transmit ////////////////////

// Gets the number of send transactions.
func (sm *ServerMetrics) GetSentTransactionsCount() uint32 {
	return atomic.LoadUint32(&sm.sentTxsCount)
}

// Increments the send transactions count.
func (sm *ServerMetrics) IncrSentTransactionsCount() uint32 {
	return atomic.AddUint32(&sm.sentTxsCount, 1)
}

// Gets the number of send transaction requests count.
func (sm *ServerMetrics) GetSentTransactionRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.sentTxsReqCount)
}

// Increments the send transactions requests count.
func (sm *ServerMetrics) IncrSentTransactionRequestsCount() uint32 {
	return atomic.AddUint32(&sm.sentTxsReqCount, 1)
}

// Gets the number of sent milestone requests.
func (sm *ServerMetrics) GetSentMilestoneRequestsCount() uint32 {
	return atomic.LoadUint32(&sm.sentMilestoneReqCount)
}

// Increments the sent milestone requests count.
func (sm *ServerMetrics) IncrSentMilestoneRequestsCount() uint32 {
	return atomic.AddUint32(&sm.sentMilestoneReqCount, 1)
}

// Gets the number of sent heartbeats.
func (sm *ServerMetrics) GetSentHeartbeatsCount() uint32 {
	return atomic.LoadUint32(&sm.sentHeartbeatsCount)
}

// Increments the sent heartbeats count.
func (sm *ServerMetrics) IncrSentHeartbeatsCount() uint32 {
	return atomic.AddUint32(&sm.sentHeartbeatsCount, 1)
}

// Gets the number of packets dropped from the neighbor's send queue.
func (sm *ServerMetrics) GetDroppedSendPacketsCount() uint32 {
	return atomic.LoadUint32(&sm.droppedSendPacketsCount)
}

// Increments the number of packets dropped from the neighbor's send queue.
func (sm *ServerMetrics) IncrDroppedSendPacketsCount() uint32 {
	return atomic.AddUint32(&sm.droppedSendPacketsCount, 1)
}

//////////////////// Spammer ////////////////////

// Gets the number of sent spam txs.
func (sm *ServerMetrics) GetSentSpamTxsCount() uint32 {
	return atomic.LoadUint32(&sm.sentSpamTxsCount)
}

// Increments the sent spam txs count.
func (sm *ServerMetrics) IncrSentSpamTxsCount() uint32 {
	return atomic.AddUint32(&sm.sentSpamTxsCount, 1)
}

//////////////////// Global ////////////////////

// Gets the number of validated bundles.
func (sm *ServerMetrics) GetValidatedBundlesCount() uint32 {
	return atomic.LoadUint32(&sm.validatedBundlesCount)
}

// Increments the validated bundles count.
func (sm *ServerMetrics) IncrValidatedBundlesCount() uint32 {
	return atomic.AddUint32(&sm.validatedBundlesCount, 1)
}

// Gets the number of seen spent addresses.
func (sm *ServerMetrics) GetSeenSpentAddrCount() uint32 {
	return atomic.LoadUint32(&sm.seenSpentAddrCount)
}

// Increments the seen spent address count.
func (sm *ServerMetrics) IncrSeenSpentAddrCount() uint32 {
	return atomic.AddUint32(&sm.seenSpentAddrCount, 1)
}
