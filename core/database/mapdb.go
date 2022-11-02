package database

import (
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
)

func newMapDB(metrics *metrics.DatabaseMetrics) *database.Database {

	return database.New(
		"",
		mapdb.NewMapDB(),
		hivedb.EngineMapDB,
		metrics,
		&database.Events{
			Cleanup:    events.NewEvent(database.CleanupCaller),
			Compaction: events.NewEvent(events.BoolCaller),
		},
		false,
		nil,
	)
}
