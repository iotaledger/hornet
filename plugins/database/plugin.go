package database

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/store"
)

var (
	PLUGIN = node.NewPlugin("Database", node.Enabled, configure, run)
	log    *logger.Logger

	garbageCollectionLock syncutils.Mutex
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	tangle.ConfigureDatabases(config.NodeConfig.GetString(config.CfgDatabasePath))

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangle.MarkDatabaseHealthy()
		log.Info("Syncing database to disk...")
		store.Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

// runFullGarbageCollectionWithoutLocking does several database garbage collection runs until there was nothing to clean up.
func runFullGarbageCollectionWithoutLocking(discardRatio ...float64) (int, error) {
	cleanups := 0

	var err error
	for err == nil {
		if err = store.Cleanup(discardRatio...); err == nil {
			cleanups++
		}
	}
	return cleanups, err
}

// RunFullGarbageCollection does several database garbage collection runs until there was nothing to clean up.
func RunFullGarbageCollection(discardRatio ...float64) {
	garbageCollectionLock.Lock()
	defer garbageCollectionLock.Unlock()

	log.Info("running full database garbage collection. This can take a while...")

	start := time.Now()

	Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
		Start: start,
	})

	cleanups, err := runFullGarbageCollectionWithoutLocking(discardRatio...)

	end := time.Now()

	Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
		Start: start,
		End:   end,
	})

	if err != nil {
		if err != store.ErrNothingToCleanup {
			log.Warnf("full database garbage collection failed with error: %s. took: %v", err.Error(), end.Sub(start).Truncate(time.Millisecond))
			return
		}
	}

	log.Infof("full database garbage collection finished. cleaned up %d files. took %v", cleanups, end.Sub(start).Truncate(time.Millisecond))
}

func run(_ *node.Plugin) {
	// do nothing
}
