package database

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Database", node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	tangle.ConfigureDatabases(config.NodeConfig.GetString(config.CfgDatabasePath), &profile.GetProfile().Badger)

	if tangle.IsDatabaseCorrupted() {
		log.Panic("HORNET was not shut down correctly. Database is corrupted. Please delete the database folder and start with a new local snapshot.")
	}

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	daemon.BackgroundWorker("Database Health", func(shutdownSignal <-chan struct{}) {
		tangle.MarkDatabaseCorrupted()
	})

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		tangle.MarkDatabaseHealthy()

		log.Info("Syncing database to disk...")
		database.GetHornetBadgerInstance().Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.ShutdownPriorityCloseDatabase)
}

func run(plugin *node.Plugin) {
	// nothing
}
