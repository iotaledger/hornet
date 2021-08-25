package prometheus

import (
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/hive.go/events"
)

var (
	databaseSizeBytes prometheus.Gauge
	compactionCount   prometheus.Counter
	compactionRunning prometheus.Gauge
	pruningCount      prometheus.Counter
	pruningRunning    prometheus.Gauge
)

func configureDatabase() {

	databaseSizeBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "database",
			Name:      "size_bytes",
			Help:      "Database sizes in bytes.",
		})

	compactionCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "database",
			Name:      "compaction_count",
			Help:      "The total amount of database compactions.",
		},
	)

	deps.Database.Events().DatabaseCompaction.Attach(events.NewClosure(func(running bool) {
		if running {
			compactionCount.Inc()
		}
	}))

	compactionRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "iota",
		Subsystem: "database",
		Name:      "compaction_running",
		Help:      "Current state of database compaction process.",
	})

	pruningCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "database",
			Name:      "pruning_count",
			Help:      "The total amount of database prunings.",
		},
	)

	deps.Storage.Events.PruningStateChanged.Attach(events.NewClosure(func(running bool) {
		if running {
			pruningCount.Inc()
		}
	}))

	pruningRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "iota",
		Subsystem: "database",
		Name:      "pruning_running",
		Help:      "Current state of database pruning process.",
	})

	registry.MustRegister(databaseSizeBytes)
	registry.MustRegister(compactionCount)
	registry.MustRegister(compactionRunning)
	registry.MustRegister(pruningCount)
	registry.MustRegister(pruningRunning)

	addCollect(collectDatabase)
}

func collectDatabase() {
	databaseSizeBytes.Set(0)
	dbSize, err := directorySize(deps.DatabasePath)
	if err == nil {
		databaseSizeBytes.Set(float64(dbSize))
	}

	compactionRunning.Set(0)
	if deps.Database.CompactionRunning() {
		compactionRunning.Set(1)
	}

	pruningRunning.Set(0)
	if deps.StorageMetrics.PruningRunning.Load() {
		pruningRunning.Set(1)
	}
}

func directorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
