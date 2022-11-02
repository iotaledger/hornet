package database

import (
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/kvstore/pebble"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
)

func newPebble(path string, metrics *metrics.DatabaseMetrics) *database.Database {

	dbEvents := &database.Events{
		Cleanup:    events.NewEvent(database.CleanupCaller),
		Compaction: events.NewEvent(events.BoolCaller),
	}

	reportCompactionRunning := func(running bool) {
		metrics.CompactionRunning.Store(running)
		if running {
			metrics.CompactionCount.Inc()
		}
		dbEvents.Compaction.Trigger(running)
	}

	db, err := database.NewPebbleDB(path, reportCompactionRunning, true)
	if err != nil {
		CoreComponent.LogPanicf("pebble database initialization failed: %s", err)
	}

	return database.New(
		path,
		pebble.New(db),
		hivedb.EnginePebble,
		metrics,
		dbEvents,
		true,
		func() bool {
			return metrics.CompactionRunning.Load()
		},
	)

}
