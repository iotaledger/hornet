package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
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

	belowMaxDepthTransactionLimit = config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepthTransactionLimit)
	configureRefsAnInvalidBundleStorage()

	tangle.ConfigureMilestones(
		config.NodeConfig.GetString(config.CfgMilestoneCoordinator),
		config.NodeConfig.GetInt(config.CfgMilestoneCoordinatorSecurityLevel),
		uint64(config.NodeConfig.GetInt(config.CfgMilestoneNumberOfKeysInAMilestone)),
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
		tangle.ShutdownSpentAddressesStorage()
		log.Info("Flushing caches to database... done")

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
