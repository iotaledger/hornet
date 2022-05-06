package tangle

import "github.com/iotaledger/hive.go/math"

func (t *Tangle) LastConfirmedMilestoneMetric() *ConfirmedMilestoneMetric {
	t.lastConfirmedMilestoneMetricLock.RLock()
	defer t.lastConfirmedMilestoneMetricLock.RUnlock()

	return t.lastConfirmedMilestoneMetric
}

// measures the MPS values
func (t *Tangle) measureMPS() {
	incomingMsgCnt := t.serverMetrics.Messages.Load()
	incomingNewMsgCnt := t.serverMetrics.NewMessages.Load()
	outgoingMsgCnt := t.serverMetrics.SentMessages.Load()

	mpsMetrics := &MPSMetrics{
		Incoming: math.Uint32Diff(incomingMsgCnt, t.lastIncomingMsgCnt),
		New:      math.Uint32Diff(incomingNewMsgCnt, t.lastIncomingNewMsgCnt),
		Outgoing: math.Uint32Diff(outgoingMsgCnt, t.lastOutgoingMsgCnt),
	}

	// store the new counters
	t.lastIncomingMsgCnt = incomingMsgCnt
	t.lastIncomingNewMsgCnt = incomingNewMsgCnt
	t.lastOutgoingMsgCnt = outgoingMsgCnt

	// trigger events for outside listeners
	t.Events.MPSMetricsUpdated.Trigger(mpsMetrics)
}
