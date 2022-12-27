package database

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
	"github.com/iotaledger/hornet/pkg/database"
	"github.com/iotaledger/hornet/pkg/metrics"
)

func newRocksDB(path string, metrics *metrics.DatabaseMetrics) *database.Database {

	events := &database.Events{
		DatabaseCleanup:    events.NewEvent(database.DatabaseCleanupCaller),
		DatabaseCompaction: events.NewEvent(events.BoolCaller),
	}

	rocksDatabase, err := database.NewRocksDB(path)
	if err != nil {
		CorePlugin.LogPanicf("rocksdb database initialization failed: %s", err)
	}

	database := database.New(
		path,
		rocksdb.New(rocksDatabase),
		database.EngineRocksDB,
		metrics,
		events,
		true,
		func() bool {
			if numCompactions, success := rocksDatabase.GetIntProperty("rocksdb.num-running-compactions"); success {
				runningBefore := metrics.CompactionRunning.Load()
				running := numCompactions != 0

				metrics.CompactionRunning.Store(running)
				if running && !runningBefore {
					// we may miss some compactions, since this is only calculated if polled.
					metrics.CompactionCount.Inc()
					events.DatabaseCompaction.Trigger(running)
				}
				return running
			}
			return false
		},
	)

	return database

}
