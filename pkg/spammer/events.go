package spammer

import (
	"github.com/iotaledger/hive.go/events"
)

// SpamStats are stats for a single spam block.
type SpamStats struct {
	Tipselection float32 `json:"tipselect"`
	ProofOfWork  float32 `json:"pow"`
}

// AvgSpamMetrics are average metrics of the created spam.
type AvgSpamMetrics struct {
	NewBlocks              uint32  `json:"newBlocks"`
	AverageBlocksPerSecond float32 `json:"avgBlocks"`
}

// SpammerEvents are the events issued by the spammer.
type SpammerEvents struct {
	// Fired when a single spam block is issued.
	SpamPerformed *events.Event
	// Fired when average spam metrics were updated by the worker.
	AvgSpamMetricsUpdated *events.Event
}

// SpamStatsCaller is used to signal new SpamStats.
func SpamStatsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*SpamStats))(params[0].(*SpamStats))
}

// AvgSpamMetricsCaller is used to signal new AvgSpamMetrics.
func AvgSpamMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*AvgSpamMetrics))(params[0].(*AvgSpamMetrics))
}
