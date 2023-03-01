package database

import (
	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
)

func newMapDB(metrics *metrics.DatabaseMetrics) *database.Database {
	return database.New(
		"",
		mapdb.NewMapDB(),
		hivedb.EngineMapDB,
		metrics,
		database.NewEvents(),
		false,
		nil,
	)
}
