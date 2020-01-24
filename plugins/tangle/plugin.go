package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/lru_cache"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	hornetDB "github.com/gohornet/hornet/packages/database"
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
	RefsAnInvalidBundleCache = lru_cache.NewLRUCache(profile.GetProfile().Caches.RefsInvalidBundle.Size)

	tangle.InitBundleCache()
	tangle.InitMilestoneCache()

	tangle.ConfigureDatabases(parameter.NodeConfig.GetString("db.path"), &profile.GetProfile().Badger)

	tangle.InitSpentAddressesCuckooFilter()

	log.Infof("spent addresses Cuckoo filter contains %d addresses", tangle.CountSpentAddressesEntries())

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
		tangle.FlushMilestoneCache()
		tangle.FlushBundleCache()
		tangle.ShutdownTransactionStorage()
		tangle.ShutdownApproversStorage()
		if err := tangle.StoreSpentAddressesCuckooFilterInDatabase(); err != nil {
			log.Panicf("couldn't persist spent addresses cuckoo filter: %s", err.Error())
		}
		log.Info("Flushing caches to database... done")

		tangle.MarkDatabaseHealthy()

		log.Info("Syncing database to disk...")
		hornetDB.GetHornetBadgerInstance().Close()
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
		timeutil.Ticker(hornetDB.CleanupHornetBadgerInstance, 5*time.Minute, shutdownSignal)
	}, shutdown.ShutdownPriorityBadgerGarbageCollection)
}
