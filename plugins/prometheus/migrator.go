package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	migratorHealth        *prometheus.GaugeVec
	migratorMsIndex       *prometheus.GaugeVec
	migratorIncludedIndex *prometheus.GaugeVec
)

func configureMigrator() {
	migratorHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_migrator_health",
			Help: "The migrator service's health.",
		},
		[]string{"name"},
	)
	migratorMsIndex = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_migrator_ms_index",
			Help: "The migrator's current legacy milestone index to migrate.",
		},
		[]string{"name"},
	)
	migratorIncludedIndex = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_migrator_included_index",
			Help: "The migrator's current included index/offset of the funds to migrate for a given legacy milestone.",
		},
		[]string{"name"},
	)

	registry.MustRegister(migratorHealth)
	registry.MustRegister(migratorMsIndex)
	registry.MustRegister(migratorIncludedIndex)

	addCollect(collectMigrator)
}

func collectMigrator() {
	migratorHealth.Reset()
	migratorHealth.WithLabelValues("health").Set(func() float64 {
		if deps.MigratorService.Healthy.Load() {
			return 1
		}
		return 0
	}())
	migratorMsIndex.WithLabelValues()
}
