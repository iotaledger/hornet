package dashboard

import (
	"encoding/json"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/database"
)

var (
	lastDbCleanup       = &database.DatabaseCleanup{}
	cachedDbSizeMetrics []*dbSize
)

type dbSize struct {
	Tangle   int64
	Snapshot int64
	Spent    int64
	Time     time.Time
}

func (s *dbSize) MarshalJSON() ([]byte, error) {
	size := struct {
		Tangle   int64 `json:"tangle"`
		Snapshot int64 `json:"snapshot"`
		Spent    int64 `json:"spent"`
		Time     int64 `json:"ts"`
	}{
		Tangle:   s.Tangle,
		Snapshot: s.Snapshot,
		Spent:    s.Spent,
		Time:     s.Time.Unix(),
	}

	return json.Marshal(size)
}

func currentDatabaseSize() *dbSize {
	tangle, snapshot, spent := tangle.GetDatabaseSizes()
	newValue := &dbSize{
		Tangle:   tangle,
		Snapshot: snapshot,
		Spent:    spent,
		Time:     time.Now(),
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
		hub.BroadcastMsg(&msg{Type: MsgTypeDatabaseCleanupEvent, Data: cleanup})
	})

	daemon.BackgroundWorker("Dashboard[DBSize]", func(shutdownSignal <-chan struct{}) {
		database.Events.DatabaseCleanup.Attach(onDatabaseCleanup)
		defer database.Events.DatabaseCleanup.Detach(onDatabaseCleanup)

		timeutil.Ticker(func() {
			dbSizeMetric := currentDatabaseSize()
			hub.BroadcastMsg(&msg{Type: MsgTypeDatabaseSizeMetric, Data: []*dbSize{dbSizeMetric}})
		}, 1*time.Minute, shutdownSignal)
	}, shutdown.PriorityDashboard)
}
