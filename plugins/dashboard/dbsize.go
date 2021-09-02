package dashboard

import (
	"encoding/json"
	"time"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"
)

var (
	lastDBCleanup       = &database.DatabaseCleanup{}
	cachedDBSizeMetrics []*DBSizeMetric
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
	dbSize, err := deps.Storage.DatabaseSize()
	if err != nil {
		Plugin.LogWarnf("error in database size calculation: %s", err)
		return nil
	}

	newValue := &DBSizeMetric{
		Total: dbSize,
		Time:  time.Now(),
	}
	cachedDBSizeMetrics = append(cachedDBSizeMetrics, newValue)
	if len(cachedDBSizeMetrics) > 600 {
		cachedDBSizeMetrics = cachedDBSizeMetrics[len(cachedDBSizeMetrics)-600:]
	}
	return newValue
}

func runDatabaseSizeCollector() {

	// Gather first metric so we have a starting point
	currentDatabaseSize()

	onDatabaseCleanup := events.NewClosure(func(cleanup *database.DatabaseCleanup) {
		lastDBCleanup = cleanup
		hub.BroadcastMsg(&Msg{Type: MsgTypeDatabaseCleanupEvent, Data: cleanup})
	})

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[DBSize]", func(shutdownSignal <-chan struct{}) {
		deps.Database.Events().DatabaseCleanup.Attach(onDatabaseCleanup)
		defer deps.Database.Events().DatabaseCleanup.Detach(onDatabaseCleanup)

		ticker := timeutil.NewTicker(func() {
			dbSizeMetric := currentDatabaseSize()
			hub.BroadcastMsg(&Msg{Type: MsgTypeDatabaseSizeMetric, Data: []*DBSizeMetric{dbSizeMetric}})
		}, 1*time.Minute, shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityDashboard); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}
