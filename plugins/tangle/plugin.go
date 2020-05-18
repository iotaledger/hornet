package tangle

import (
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/database"
	"github.com/gohornet/hornet/plugins/gossip"
)

var (
	PLUGIN                        = node.NewPlugin("Tangle", node.Enabled, configure, run)
	belowMaxDepthTransactionLimit int
	log                           *logger.Logger
	updateSyncedAtStartup         bool

	syncedAtStartup = pflag.Bool("syncedAtStartup", false, "LMI is set to LSMI at startup")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new local snapshot.")
)

func init() {
	pflag.CommandLine.MarkHidden("syncedAtStartup")
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	belowMaxDepthTransactionLimit = config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepthTransactionLimit)
	configureRefsAnInvalidBundleStorage()

	tangle.LoadInitialValuesFromDatabase()

	updateSyncedAtStartup = *syncedAtStartup

	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	daemon.BackgroundWorker("Database Health", func(shutdownSignal <-chan struct{}) {
		tangle.MarkDatabaseCorrupted()
	})

	tangle.ConfigureMilestones(
		config.NodeConfig.GetString(config.CfgCoordinatorAddress),
		config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel),
		uint64(config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth)),
	)

	daemon.BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		abortMilestoneSolidification()

		log.Info("Flushing caches to database...")
		tangle.ShutdownMilestoneStorage()
		tangle.ShutdownBundleStorage()
		tangle.ShutdownBundleTransactionsStorage()
		tangle.ShutdownTransactionStorage()
		tangle.ShutdownApproversStorage()
		tangle.ShutdownTagsStorage()
		tangle.ShutdownAddressStorage()
		tangle.ShutdownUnconfirmedTxsStorage()
		tangle.ShutdownSpentAddressesStorage()
		log.Info("Flushing caches to database... done")

	}, shutdown.PriorityFlushToDatabase)

	Events.SolidMilestoneChanged.Attach(events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		defer cachedBndl.Release() // bundle -1
		// notify peers about our new solid milestone index
		gossip.BroadcastHeartbeat()
	}))

	Events.PruningMilestoneIndexChanged.Attach(events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new pruning milestone index
		gossip.BroadcastHeartbeat()
	}))

	configureTangleProcessor(plugin)

	gossip.AddRequestBackpressureSignal(IsReceiveTxWorkerPoolBusy)
}

func run(plugin *node.Plugin) {

	if tangle.IsDatabaseCorrupted() {
		log.Warnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := revalidateDatabase(); err != nil {
			log.Panic(errors.Wrap(ErrDatabaseRevalidationFailed, err.Error()))
		}
		log.Info("database revalidation successful")
	}

	// run a full database garbage collection at startup
	database.RunGarbageCollection()

	tangle.SetLatestMilestoneIndex(tangle.GetSolidMilestoneIndex(), updateSyncedAtStartup)

	runTangleProcessor(plugin)

	// create a background worker that prints a status message every second
	daemon.BackgroundWorker("Tangle status reporter", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityStatusReport)
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func SetUpdateSyncedAtStartup(updateSynced bool) {
	updateSyncedAtStartup = updateSynced
}
