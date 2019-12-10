package metrics

import (
	"github.com/gohornet/hornet/plugins/gossip/server"
)

var (
	lastIncomingTxCnt    uint32
	lastIncomingNewTxCnt uint32
	lastOutgoingTxCnt    uint32
)

// measures the TPS values
func measureTPS() {
	incomingTxCnt := server.SharedServerMetrics.GetAllTransactionsCount()
	incomingNewTxCnt := server.SharedServerMetrics.GetNewTransactionsCount()
	outgoingTxCnt := server.SharedServerMetrics.GetSentTransactionsCount()

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
