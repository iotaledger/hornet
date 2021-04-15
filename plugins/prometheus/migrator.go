package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	migratorSoftErrEncountered     prometheus.Counter
	receiptCount                   prometheus.Counter
	receiptMigrationEntriesApplied prometheus.Counter
)

func configureMigrator() {
	migratorSoftErrEncountered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "migrator",
			Name:      "soft_error_count",
			Help:      "The migrator service's encountered soft error count.",
		},
	)

	registry.MustRegister(migratorSoftErrEncountered)

	deps.MigratorService.Events.SoftError.Attach(events.NewClosure(func(_ error) {
		migratorSoftErrEncountered.Inc()
	}))
}

func configureReceipts() {
	receiptCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "migrator",
			Name:      "receipt_count",
			Help:      "The count of encountered receipts.",
		},
	)

	receiptMigrationEntriesApplied = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "migrator",
			Name:      "receipt_entries_applied_count",
			Help:      "The count of migration entries applied through receipts.",
		},
	)

	registry.MustRegister(receiptCount)
	registry.MustRegister(receiptMigrationEntriesApplied)

	deps.Tangle.Events.NewReceipt.Attach(events.NewClosure(func(r *iotago.Receipt) {
		receiptCount.Inc()
		receiptMigrationEntriesApplied.Add(float64(len(r.Funds)))
	}))
}
