package metrics

import (
	"github.com/gohornet/hornet/packages/metrics"
)

var (
	lastIncomingTxCnt    uint32
	lastIncomingNewTxCnt uint32
	lastOutgoingTxCnt    uint32
)

// measures the TPS values
func measureTPS() {
	incomingTxCnt := metrics.SharedServerMetrics.GetAllTransactionsCount()
	incomingNewTxCnt := metrics.SharedServerMetrics.GetNewTransactionsCount()
	outgoingTxCnt := metrics.SharedServerMetrics.GetSentTransactionsCount()

	tpsMetrics := &TPSMetrics{
		Incoming: incomingTxCnt - lastIncomingTxCnt,
		New:      incomingNewTxCnt - lastIncomingNewTxCnt,
		Outgoing: outgoingTxCnt - lastOutgoingTxCnt,
	}

	// store the new counters
	lastIncomingTxCnt = incomingTxCnt
	lastIncomingNewTxCnt = incomingNewTxCnt
	lastOutgoingTxCnt = outgoingTxCnt

	// trigger events for outside listeners
	Events.TPSMetricsUpdated.Trigger(tpsMetrics)

	// DEBUG
	//gossip.DebugPrintQueueStats()
}
