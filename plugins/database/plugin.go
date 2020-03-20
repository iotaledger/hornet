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

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		// Only mark database as healthy, if there was no revalidation or it was finished already
		revalidationIndex := tangle.GetSnapshotInfo().RevalidationIndex
		if revalidationIndex == 0 || tangle.GetSolidMilestoneIndex() > revalidationIndex {
			tangle.MarkDatabaseHealthy()
		}

		log.Info("Syncing database to disk...")
		database.GetHornetBadgerInstance().Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.ShutdownPriorityCloseDatabase)
}

func run(plugin *node.Plugin) {
	// nothing
}
