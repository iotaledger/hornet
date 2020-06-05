package database

import (
	"runtime"
	"time"

	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Database", node.Enabled, configure, run)
	log    *logger.Logger

	garbageCollectionLock syncutils.Mutex
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	viper.BindEnv("GOMAXPROCS")
	goMaxProcsEnv := viper.GetInt("GOMAXPROCS")
	if goMaxProcsEnv == 0 {
		// badger documentation recommends setting a high number for GOMAXPROCS.
		// this allows Go to observe the full IOPS throughput provided by modern SSDs.
		// Dgraph uses 128.
		runtime.GOMAXPROCS(128)
	}

	tangle.ConfigureDatabases(config.NodeConfig.GetString(config.CfgDatabasePath))

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangle.MarkDatabaseHealthy()
		log.Info("Syncing databases to disk...")
		tangle.CloseDatabases()
		log.Info("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

func RunGarbageCollection() {
	if tangle.DatabaseSupportsCleanup() {

		garbageCollectionLock.Lock()
		defer garbageCollectionLock.Unlock()

		log.Info("running full database garbage collection. This can take a while...")

		start := time.Now()

		Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
			Start: start,
		})

		err := tangle.CleanupDatabases()

		end := time.Now()

		Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
			Start: start,
			End:   end,
		})

		if err != nil {
			if err != tangle.ErrNothingToCleanUp {
				log.Warnf("full database garbage collection failed with error: %s. took: %v", err.Error(), end.Sub(start).Truncate(time.Millisecond))
				return
			}
		}

		log.Infof("full database garbage collection finished. took %v", end.Sub(start).Truncate(time.Millisecond))
	}
}

func run(_ *node.Plugin) {
	// do nothing
}
