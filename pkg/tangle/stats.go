package tangle

import "github.com/iotaledger/hive.go/core/math"

func (t *Tangle) LastConfirmedMilestoneMetric() *ConfirmedMilestoneMetric {
	t.lastConfirmedMilestoneMetricLock.RLock()
	defer t.lastConfirmedMilestoneMetricLock.RUnlock()

	return t.lastConfirmedMilestoneMetric
}

// measures the BPS values.
func (t *Tangle) measureBPS() {
	incomingBlocksCount := t.serverMetrics.Blocks.Load()
	incomingNewBlocksCount := t.serverMetrics.NewBlocks.Load()
	outgoingBlocksCount := t.serverMetrics.SentBlocks.Load()

	bpsMetrics := &BPSMetrics{
		Incoming: math.Uint32Diff(incomingBlocksCount, t.lastIncomingBlocksCount),
		New:      math.Uint32Diff(incomingNewBlocksCount, t.lastIncomingNewBlocksCount),
		Outgoing: math.Uint32Diff(outgoingBlocksCount, t.lastOutgoingBlocksCount),
	}

	// store the new counters
	t.lastIncomingBlocksCount = incomingBlocksCount
	t.lastIncomingNewBlocksCount = incomingNewBlocksCount
	t.lastOutgoingBlocksCount = outgoingBlocksCount

	// trigger events for outside listeners
	t.Events.BPSMetricsUpdated.Trigger(bpsMetrics)
}
