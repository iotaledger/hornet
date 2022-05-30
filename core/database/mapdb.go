package database

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hornet/pkg/database"
	"github.com/iotaledger/hornet/pkg/metrics"
)

func newMapDB(metrics *metrics.DatabaseMetrics) *database.Database {

	return database.New(
		"",
		mapdb.NewMapDB(),
		database.EngineMapDB,
		metrics,
		&database.Events{
			DatabaseCleanup:    events.NewEvent(database.DatabaseCleanupCaller),
			DatabaseCompaction: events.NewEvent(events.BoolCaller),
		},
		false,
		nil,
	)
}
