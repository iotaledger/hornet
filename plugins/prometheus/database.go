package prometheus

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/events"
)

type storageMetrics struct {
	storage        *storage.Storage
	storageMetrics *metrics.StorageMetrics

	pruningCount   prometheus.Counter
	pruningRunning prometheus.Gauge
}

type databaseMetrics struct {
	database        *database.Database
	databaseMetrics *metrics.DatabaseMetrics

	databaseSizeBytes prometheus.Gauge
	compactionCount   prometheus.Counter
	compactionRunning prometheus.Gauge
}

func configureStorage(storage *storage.Storage, metrics *metrics.StorageMetrics) {

	m := &storageMetrics{
		storage:        storage,
		storageMetrics: metrics,
	}

	m.pruningCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "database",
			Name:      "pruning_count",
			Help:      "The total amount of database prunings.",
		},
	)

	storage.Events.PruningStateChanged.Attach(events.NewClosure(func(running bool) {
		if running {
			m.pruningCount.Inc()
		}
	}))

	m.pruningRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "iota",
		Subsystem: "database",
		Name:      "pruning_running",
		Help:      "Current state of database pruning process.",
	})

	registry.MustRegister(m.pruningRunning)
	registry.MustRegister(m.pruningCount)

	addCollect(m.collect)
}

func (m *storageMetrics) collect() {

	m.pruningRunning.Set(0)
	if m.storageMetrics.PruningRunning.Load() {
		m.pruningRunning.Set(1)
	}
}

func configureDatabase(name string, db *database.Database, metrics *metrics.DatabaseMetrics) {

	m := &databaseMetrics{
		database:        db,
		databaseMetrics: metrics,
	}

	m.databaseSizeBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iota",
			Subsystem: "database",
			Name:      fmt.Sprintf("%s_size_bytes", name),
			Help:      "Database sizes in bytes.",
		})

	m.compactionCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iota",
			Subsystem: "database",
			Name:      fmt.Sprintf("%s_compaction_count", name),
			Help:      "The total amount of tangle database compactions.",
		},
	)

	db.Events().DatabaseCompaction.Attach(events.NewClosure(func(running bool) {
		if running {
			m.compactionCount.Inc()
		}
	}))

	m.compactionRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "iota",
		Subsystem: "database",
		Name:      fmt.Sprintf("%s_compaction_running", name),
		Help:      "Current state of database compaction process.",
	})

	registry.MustRegister(m.databaseSizeBytes)
	registry.MustRegister(m.compactionCount)
	registry.MustRegister(m.compactionRunning)

	addCollect(m.collect)
}

func (m *databaseMetrics) collect() {
	m.databaseSizeBytes.Set(0)
	dbSize, err := m.database.Size()
	if err == nil {
		m.databaseSizeBytes.Set(float64(dbSize))
	}

	m.compactionRunning.Set(0)
	if m.database.CompactionRunning() {
		m.compactionRunning.Set(1)
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
