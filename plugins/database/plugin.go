package database

import (
	"runtime"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Database", node.Enabled, configure, run)
	log    *logger.Logger

	garbageCollectionLock syncutils.Mutex

	useBolt = true
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

	badgerOpts := profile.LoadProfile().Badger
	if config.NodeConfig.GetBool(config.CfgDatabaseDebugLog) {
		badgerOpts.Logger = NewBadgerLogger()
	}
	tangle.ConfigureDatabases(config.NodeConfig.GetString(config.CfgDatabasePath), &badgerOpts, useBolt)

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangle.MarkDatabaseHealthy()
		log.Info("Syncing database to disk...")
		database.Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

// runFullGarbageCollectionWithoutLocking does several database garbage collection runs until there was nothing to clean up.
func runFullGarbageCollectionWithoutLocking(discardRatio ...float64) (int, error) {
	cleanups := 0

	var err error
	for err == nil {
		if err = database.Cleanup(discardRatio...); err == nil {
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
		if err != badger.ErrNoRewrite {
			log.Warnf("full database garbage collection failed with error: %s. took: %v", err.Error(), end.Sub(start).Truncate(time.Millisecond))
			return
		}
	}

	log.Infof("full database garbage collection finished. cleaned up %d files. took %v", cleanups, end.Sub(start).Truncate(time.Millisecond))
}

func run(_ *node.Plugin) {
	// do nothing
}
