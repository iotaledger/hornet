package database

import (
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
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
