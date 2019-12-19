package tangle

import (
	"time"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/datastructure"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/node"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/timeutil"
	"github.com/gohornet/hornet/plugins/gossip"
	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	PLUGIN                        = node.NewPlugin("Tangle", node.Enabled, configure, run)
	belowMaxDepthTransactionLimit int
)

func configure(plugin *node.Plugin) {

	belowMaxDepthTransactionLimit = parameter.NodeConfig.GetInt("tipsel.belowMaxDepthTransactionLimit")
	RefsAnInvalidBundleCache = datastructure.NewLRUCache(profile.GetProfile().Caches.RefsInvalidBundle.Size)

	tangle.InitTransactionCache(onEvictTransactions)
	tangle.InitBundleCache()
	tangle.InitApproversCache()
	tangle.InitMilestoneCache()
	tangle.InitSpentAddressesCache()

	tangle.ConfigureDatabases(parameter.NodeConfig.GetString("db.path"))

	if tangle.IsDatabaseCorrupted() {
		log.Panic("HORNET was not shut down correctly. Database is corrupted. Please delete the database folder and start with a new local snapshot.")
	}

	if !tangle.IsCorrectDatabaseVersion() {
		log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
	}

	tangle.MarkDatabaseCorrupted()

	tangle.ConfigureMilestones(
		trinary.Hash(parameter.NodeConfig.GetString("milestones.coordinator")),
		parameter.NodeConfig.GetInt("milestones.coordinatorSecurityLevel"),
		uint64(parameter.NodeConfig.GetInt("milestones.numberOfKeysInAMilestone")),
	)

	daemon.BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		log.Info("Flushing caches to database...")
		tangle.FlushMilestoneCache()
		tangle.FlushBundleCache()
		tangle.FlushTransactionCache()
		tangle.FlushApproversCache()
		tangle.FlushSpentAddressesCache()
		log.Info("Flushing caches to database... done")

		tangle.MarkDatabaseHealthy()

		log.Info("Syncing database to disk...")
		database.CloseDatabase()
		log.Info("Syncing database to disk... done")
	}, shutdown.ShutdownPriorityFlushToDatabase)

	Events.SolidMilestoneChanged.Attach(events.NewClosure(func(msBundle *tangle.Bundle) {
		// notify neighbors about our new solid milestone index
		gossip.SendHeartbeat()
		gossip.SendMilestoneRequests(msBundle.GetMilestoneIndex(), tangle.GetLatestMilestoneIndex())
	}))

	Events.SnapshotMilestoneIndexChanged.Attach(events.NewClosure(func(msIndex milestone_index.MilestoneIndex) {
		// notify neighbors about our new solid milestone index
		gossip.SendHeartbeat()
		gossip.SendMilestoneRequests(msIndex, tangle.GetLatestMilestoneIndex())
	}))

	tangle.LoadInitialValuesFromDatabase()
	configureTangleProcessor(plugin)
}

func run(plugin *node.Plugin) {
	runTangleProcessor(plugin)

	// create a background worker that prints a status message every second
	daemon.BackgroundWorker("Tangle status reporter", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
	}, shutdown.ShutdownPriorityStatusReport)

	// create a db cleanup worker
	daemon.BackgroundWorker("Badger garbage collection", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(database.CleanupBadgerInstance, 5*time.Minute, shutdownSignal)
	}, shutdown.ShutdownPriorityBadgerGarbageCollection)
}
