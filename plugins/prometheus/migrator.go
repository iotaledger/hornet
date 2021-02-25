package prometheus

import (
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	migratorSoftErrEncountered      prometheus.Counter
	migratorMigrationEntriesFetched prometheus.Counter
	receiptCount                    prometheus.Counter
	receiptMigrationEntriesApplied  prometheus.Counter
)

func configureMigrator() {
	migratorSoftErrEncountered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_migrator_soft_errors",
			Help: "The migrator service's encountered soft error count.",
		},
	)

	migratorMigrationEntriesFetched = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_migrator_migration_entries_fetched_count",
			Help: "The count of legacy migration entries fetched.",
		},
	)

	registry.MustRegister(migratorSoftErrEncountered)

	deps.MigratorService.Events.SoftError.Attach(events.NewClosure(func(_ error) {
		migratorSoftErrEncountered.Inc()
	}))

	deps.MigratorService.Events.MigratedFundsFetched.Attach(events.NewClosure(func(funds []*iotago.MigratedFundsEntry) {
		migratorMigrationEntriesFetched.Add(float64(len(funds)))
	}))
}

func configureReceipts() {
	receiptCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_receipts_count",
			Help: "The count of encountered receipts.",
		},
	)

	receiptMigrationEntriesApplied = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_receipts_entries_applied_count",
			Help: "The count of migration entries applied through receipts.",
		},
	)

	registry.MustRegister(receiptCount)
	registry.MustRegister(receiptMigrationEntriesApplied)

	deps.Tangle.Events.NewReceipt.Attach(events.NewClosure(func(r *iotago.Receipt) {
		receiptCount.Inc()
		receiptMigrationEntriesApplied.Add(float64(len(r.Funds)))
	}))
}
