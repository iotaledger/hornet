package database

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
)

func newPebble(path string, metrics *metrics.DatabaseMetrics) *database.Database {

	dbEvents := &database.Events{
		DatabaseCleanup:    events.NewEvent(database.DatabaseCleanupCaller),
		DatabaseCompaction: events.NewEvent(events.BoolCaller),
	}

	reportCompactionRunning := func(running bool) {
		metrics.CompactionRunning.Store(running)
		if running {
			metrics.CompactionCount.Inc()
		}
		dbEvents.DatabaseCompaction.Trigger(running)
	}

	db, err := database.NewPebbleDB(path, reportCompactionRunning, true)
	if err != nil {
		CoreComponent.LogPanicf("pebble database initialization failed: %s", err)
	}

	return database.New(
		path,
		pebble.New(db),
		database.EnginePebble,
		metrics,
		dbEvents,
		true,
		func() bool {
			return metrics.CompactionRunning.Load()
		},
	)

}
