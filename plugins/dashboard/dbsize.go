package dashboard

import (
	"encoding/json"
	"time"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"
)

var (
	lastDbCleanup       = &database.DatabaseCleanup{}
	cachedDbSizeMetrics []*DBSizeMetric
)

// DBSizeMetric represents database size metrics.
type DBSizeMetric struct {
	Total    int64
	Snapshot int64
	Time     time.Time
}

func (s *DBSizeMetric) MarshalJSON() ([]byte, error) {
	size := struct {
		Total int64 `json:"total"`
		Time  int64 `json:"ts"`
	}{
		Total: s.Total,
		Time:  s.Time.Unix(),
	}

	return json.Marshal(size)
}

func currentDatabaseSize() *DBSizeMetric {
	dbSize, err := deps.Storage.GetDatabaseSize()
	if err != nil {
		log.Warnf("error in GetDatabaseSize: %s", err)
		return nil
	}

	newValue := &DBSizeMetric{
		Total: dbSize,
		Time:  time.Now(),
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

	onDatabaseCleanup := events.NewClosure(func(cleanup *database.DatabaseCleanup) {
		lastDbCleanup = cleanup
		hub.BroadcastMsg(&Msg{Type: MsgTypeDatabaseCleanupEvent, Data: cleanup})
	})

	Plugin.Daemon().BackgroundWorker("Dashboard[DBSize]", func(shutdownSignal <-chan struct{}) {
		deps.DatabaseEvents.DatabaseCleanup.Attach(onDatabaseCleanup)
		defer deps.DatabaseEvents.DatabaseCleanup.Detach(onDatabaseCleanup)

		ticker := timeutil.NewTicker(func() {
			dbSizeMetric := currentDatabaseSize()
			hub.BroadcastMsg(&Msg{Type: MsgTypeDatabaseSizeMetric, Data: []*DBSizeMetric{dbSizeMetric}})
		}, 1*time.Minute, shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityDashboard)
}
