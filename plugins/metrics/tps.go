package metrics

import (
	"github.com/gohornet/hornet/pkg/metrics"
)

var (
	lastIncomingTxCnt    uint32
	lastIncomingNewTxCnt uint32
	lastOutgoingTxCnt    uint32
)

// measures the TPS values
func measureTPS() {
	incomingTxCnt := metrics.SharedServerMetrics.Transactions.Load()
	incomingNewTxCnt := metrics.SharedServerMetrics.NewTransactions.Load()
	outgoingTxCnt := metrics.SharedServerMetrics.SentTransactions.Load()

	tpsMetrics := &TPSMetrics{
		Incoming: metrics.GetUint32Diff(incomingTxCnt, lastIncomingTxCnt),
		New:      metrics.GetUint32Diff(incomingNewTxCnt, lastIncomingNewTxCnt),
		Outgoing: metrics.GetUint32Diff(outgoingTxCnt, lastOutgoingTxCnt),
	}

	// store the new counters
	lastIncomingTxCnt = incomingTxCnt
	lastIncomingNewTxCnt = incomingNewTxCnt
	lastOutgoingTxCnt = outgoingTxCnt

	// trigger events for outside listeners
	Events.TPSMetricsUpdated.Trigger(tpsMetrics)
}
