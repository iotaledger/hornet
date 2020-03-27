package metrics

import (
	"github.com/gohornet/hornet/packages/metrics"
)

var (
	lastIncomingTxCnt    uint64
	lastIncomingNewTxCnt uint64
	lastOutgoingTxCnt    uint64
)

// measures the TPS values
func measureTPS() {
	incomingTxCnt := metrics.SharedServerMetrics.Transactions.Load()
	incomingNewTxCnt := metrics.SharedServerMetrics.NewTransactions.Load()
	outgoingTxCnt := metrics.SharedServerMetrics.SentTransactions.Load()

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
}
