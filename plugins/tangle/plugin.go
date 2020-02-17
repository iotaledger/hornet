package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/profile"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
)

var (
	PLUGIN                        = node.NewPlugin("Tangle", node.Enabled, configure, run)
	belowMaxDepthTransactionLimit int
	log                           *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	belowMaxDepthTransactionLimit = parameter.NodeConfig.GetInt("tipsel.belowMaxDepthTransactionLimit")
	configureRefsAnInvalidBundleStorage()

	tangle.ConfigureDatabases(parameter.NodeConfig.GetString("db.path"), &profile.GetProfile().Badger)

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

	tangle.ConfigureMilestones(
		trinary.Hash(parameter.NodeConfig.GetString("milestones.coordinator")),
		parameter.NodeConfig.GetInt("milestones.coordinatorSecurityLevel"),
		uint64(parameter.NodeConfig.GetInt("milestones.numberOfKeysInAMilestone")),
	)

	daemon.BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		log.Info("Flushing caches to database...")
		tangle.ShutdownMilestoneStorage()
		tangle.ShutdownBundleStorage()
		tangle.ShutdownBundleTransactionsStorage()
		tangle.ShutdownTransactionStorage()
		tangle.ShutdownApproversStorage()
		tangle.ShutdownTagsStorage()
		tangle.ShutdownAddressStorage()
		tangle.ShutdownFirstSeenTxsStorage()
		log.Info("Flushing caches to database... done")

		tangle.MarkDatabaseHealthy()

		log.Info("Syncing database to disk...")
		database.GetHornetBadgerInstance().Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.ShutdownPriorityFlushToDatabase)

	Events.SolidMilestoneChanged.Attach(events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		// notify neighbors about our new solid milestone index
		gossip.SendHeartbeat()
		gossip.SendMilestoneRequests(cachedBndl.GetBundle().GetMilestoneIndex(), tangle.GetLatestMilestoneIndex())
		cachedBndl.Release() // bundle -1
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
		timeutil.Ticker(database.CleanupHornetBadgerInstance, 5*time.Minute, shutdownSignal)
	}, shutdown.ShutdownPriorityBadgerGarbageCollection)
}
