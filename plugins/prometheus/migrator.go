package prometheus

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	migratorSoftErrEncountered prometheus.Counter
)

func configureMigrator() {
	migratorSoftErrEncountered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "iota_migrator_soft_errors",
			Help: "The migrator service's encountered soft error count.",
		},
	)

	registry.MustRegister(migratorSoftErrEncountered)

	deps.MigratorService.Events.SoftError.Attach(events.NewClosure(func(_ error) {
		migratorSoftErrEncountered.Inc()
	}))
}
