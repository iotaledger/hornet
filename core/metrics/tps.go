package metrics

import (
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/utils"
)

var (
	lastIncomingMsgCnt    uint32
	lastIncomingNewMsgCnt uint32
	lastOutgoingMsgCnt    uint32
)

// measures the MPS values
func measureMPS() {
	incomingMsgCnt := metrics.SharedServerMetrics.Messages.Load()
	incomingNewMsgCnt := metrics.SharedServerMetrics.NewMessages.Load()
	outgoingMsgCnt := metrics.SharedServerMetrics.SentMessages.Load()

	mpsMetrics := &MPSMetrics{
		Incoming: utils.GetUint32Diff(incomingMsgCnt, lastIncomingMsgCnt),
		New:      utils.GetUint32Diff(incomingNewMsgCnt, lastIncomingNewMsgCnt),
		Outgoing: utils.GetUint32Diff(outgoingMsgCnt, lastOutgoingMsgCnt),
	}

	// store the new counters
	lastIncomingMsgCnt = incomingMsgCnt
	lastIncomingNewMsgCnt = incomingNewMsgCnt
	lastOutgoingMsgCnt = outgoingMsgCnt

	// trigger events for outside listeners
	Events.MPSMetricsUpdated.Trigger(mpsMetrics)
}
