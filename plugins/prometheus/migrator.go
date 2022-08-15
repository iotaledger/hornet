package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/core/events"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	receiptCount                   prometheus.Counter
	receiptMigrationEntriesApplied prometheus.Counter
)

func configureReceipts() {
	receiptCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "migrator",
			Name:      "receipt_count_total",
			Help:      "The count of encountered receipts.",
		},
	)

	receiptMigrationEntriesApplied = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "migrator",
			Name:      "receipt_entries_applied_count_total",
			Help:      "The count of migration entries applied through receipts.",
		},
	)

	registry.MustRegister(receiptCount)
	registry.MustRegister(receiptMigrationEntriesApplied)

	deps.Tangle.Events.NewReceipt.Hook(events.NewClosure(func(r *iotago.ReceiptMilestoneOpt) {
		receiptCount.Inc()
		receiptMigrationEntriesApplied.Add(float64(len(r.Funds)))
	}))
}
