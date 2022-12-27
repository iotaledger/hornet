package database

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hornet/pkg/database"
	"github.com/iotaledger/hornet/pkg/metrics"
)

func newMapDB(metrics *metrics.DatabaseMetrics) *database.Database {

	events := &database.Events{
		DatabaseCleanup:    events.NewEvent(database.DatabaseCleanupCaller),
		DatabaseCompaction: events.NewEvent(events.BoolCaller),
	}

	return database.New(
		"",
		mapdb.NewMapDB(),
		database.EngineMapDB,
		metrics,
		events,
		false,
		nil,
	)
}
