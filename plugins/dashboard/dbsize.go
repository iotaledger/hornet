package dashboard

import (
	"encoding/json"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/shutdown"
	databaseplugin "github.com/gohornet/hornet/plugins/database"
)

var (
	lastDbCleanup       = &databaseplugin.DatabaseCleanup{}
	cachedDbSizeMetrics []*dbSize
)

type dbSize struct {
	Keys   int64
	Values int64
	Time   time.Time
}

func (c *dbSize) MarshalJSON() ([]byte, error) {
	cleanup := struct {
		Keys   int64 `json:"keys"`
		Values int64 `json:"values"`
		Time   int64 `json:"ts"`
	}{
		Keys:   c.Keys,
		Values: c.Values,
		Time:   c.Time.Unix(),
	}

	return json.Marshal(cleanup)
}

func currentDatabaseSize() *dbSize {
	keys, values := database.GetDatabaseSize()
	newValue := &dbSize{
		Keys:   keys,
		Values: values,
		Time:   time.Now(),
	}
	cachedDbSizeMetrics = append(cachedDbSizeMetrics, newValue)
	if len(cachedDbSizeMetrics) > 600 {
		cachedDbSizeMetrics = cachedDbSizeMetrics[len(cachedDbSizeMetrics)-600:]
	}
	return newValue
}

func runDatabaseSizeCollector() {

	// Gather first metric so we have a starting point
	currentDatabaseSize()

	notifyDatabaseCleanup := events.NewClosure(func(cleanup *databaseplugin.DatabaseCleanup) {
		lastDbCleanup = cleanup
		wsSendWorkerPool.TrySubmit(cleanup)
	})

	daemon.BackgroundWorker("Dashboard[DBSize]", func(shutdownSignal <-chan struct{}) {
		databaseplugin.Events.DatabaseCleanup.Attach(notifyDatabaseCleanup)
		timeutil.Ticker(func() {
			dbSizeMetric := currentDatabaseSize()
			wsSendWorkerPool.TrySubmit([]*dbSize{dbSizeMetric})
		}, 1*time.Minute, shutdownSignal)
		databaseplugin.Events.DatabaseCleanup.Detach(notifyDatabaseCleanup)
	}, shutdown.PriorityDashboard)
}
