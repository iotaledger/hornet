package spammer

import (
	"github.com/iotaledger/hive.go/events"
)

// SpamStats are stats for a single spam message.
type SpamStats struct {
	Tipselection float32 `json:"tipselect"`
	ProofOfWork  float32 `json:"pow"`
}

// AvgSpamMetrics are average metrics of the created spam.
type AvgSpamMetrics struct {
	NewMessages              uint32  `json:"newMsgs"`
	AverageMessagesPerSecond float32 `json:"avgMsgs"`
}

// SpammerEvents are the events issued by the spammer.
type SpammerEvents struct {
	// Fired when a single spam message is issued.
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
