package database

import (
	"time"
	
	"github.com/dgraph-io/badger/v2"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Database", node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	badgerOpts := profile.LoadProfile().Badger
	if config.NodeConfig.GetBool(config.CfgDatabaseDebugLog) {
		badgerOpts.Logger = NewBadgerLogger()
	}
	tangle.ConfigureDatabases(config.NodeConfig.GetString(config.CfgDatabasePath), &badgerOpts)

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	// create a db cleanup worker
	daemon.BackgroundWorker("Badger garbage collection", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(func() {

			log.Info("Run badger garbage collection")
			start := time.Now()
			cleanup := &DatabaseCleanup{
				Start: start,
			}
			Events.DatabaseCleanup.Trigger(cleanup)
			err := database.CleanupHornetBadgerInstance()
			end := time.Now()
			cleanup = &DatabaseCleanup{
				Start: start,
				End:   end,
			}
			Events.DatabaseCleanup.Trigger(cleanup)
			if err != nil {
				if err != badger.ErrNoRewrite {
					log.Errorf("Badger garbage collection finished with error: %s. Took: %s", err.Error(), time.Since(start).String())
				} else {
					log.Infof("Badger garbage collection finished with nothing to clean up. Took: %s", end.Sub(start).String())
				}
			} else {
				log.Infof("Badger garbage collection finished. Took: %s", end.Sub(start).String())
			}

		}, 1*time.Minute, shutdownSignal)
	}, shutdown.PriorityBadgerGarbageCollection)

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangle.MarkDatabaseHealthy()
		log.Info("Syncing database to disk...")
		database.GetHornetBadgerInstance().Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

func run(_ *node.Plugin) {
	// nothing
}
