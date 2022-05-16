package dashboard

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"
)

var (
	lastDBCleanup       = &database.DatabaseCleanup{}
	cachedDBSizeMetrics []*DBSizeMetric
)

// DBSizeMetric represents database size metrics.
type DBSizeMetric struct {
	Tangle int64
	UTXO   int64
	Total  int64
	Time   time.Time
}

func (s *DBSizeMetric) MarshalJSON() ([]byte, error) {
	size := struct {
		Tangle int64 `json:"tangle"`
		UTXO   int64 `json:"utxo"`
		Total  int64 `json:"total"`
		Time   int64 `json:"ts"`
	}{
		Tangle: s.Tangle,
		UTXO:   s.UTXO,
		Total:  s.Total,
		Time:   s.Time.Unix(),
	}

	return json.Marshal(size)
}

func currentDatabaseSize() *DBSizeMetric {

	tangleDbSize, err := deps.TangleDatabase.Size()
	if err != nil {
		Plugin.LogWarnf("error in tangle database size calculation: %s", err)
		return nil
	}

	utxoDbSize, err := deps.UTXODatabase.Size()
	if err != nil {
		Plugin.LogWarnf("error in utxo database size calculation: %s", err)
		return nil
	}

	newValue := &DBSizeMetric{
		Tangle: tangleDbSize,
		UTXO:   utxoDbSize,
		Total:  tangleDbSize + utxoDbSize,
		Time:   time.Now(),
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

	if err := Plugin.Daemon().BackgroundWorker("Dashboard[DBSize]", func(ctx context.Context) {
		deps.TangleDatabase.Events().DatabaseCleanup.Attach(onDatabaseCleanup)
		defer deps.TangleDatabase.Events().DatabaseCleanup.Detach(onDatabaseCleanup)

		deps.UTXODatabase.Events().DatabaseCleanup.Attach(onDatabaseCleanup)
		defer deps.UTXODatabase.Events().DatabaseCleanup.Detach(onDatabaseCleanup)

		ticker := timeutil.NewTicker(func() {
			dbSizeMetric := currentDatabaseSize()
			hub.BroadcastMsg(&Msg{Type: MsgTypeDatabaseSizeMetric, Data: []*DBSizeMetric{dbSizeMetric}})
		}, 1*time.Minute, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PriorityDashboard); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
