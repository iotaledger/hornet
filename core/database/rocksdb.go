package database

import (
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/kvstore/rocksdb"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
)

func newRocksDB(path string, metrics *metrics.DatabaseMetrics) *database.Database {

	dbEvents := &database.Events{
		Cleanup:    events.NewEvent(database.CleanupCaller),
		Compaction: events.NewEvent(events.BoolCaller),
	}

	rocksDatabase, err := database.NewRocksDB(path)
	if err != nil {
		CoreComponent.LogPanicf("rocksdb database initialization failed: %s", err)
	}

	return database.New(
		path,
		rocksdb.New(rocksDatabase),
		hivedb.EngineRocksDB,
		metrics,
		dbEvents,
		true,
		func() bool {
			if numCompactions, success := rocksDatabase.GetIntProperty("rocksdb.num-running-compactions"); success {
				runningBefore := metrics.CompactionRunning.Load()
				running := numCompactions != 0

				metrics.CompactionRunning.Store(running)
				if running && !runningBefore {
					// we may miss some compactions, since this is only calculated if polled.
					metrics.CompactionCount.Inc()
					dbEvents.Compaction.Trigger(running)
				}

				return running
			}

			return false
		},
	)
}
