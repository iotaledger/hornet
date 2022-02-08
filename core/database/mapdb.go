package database

import (
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
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
