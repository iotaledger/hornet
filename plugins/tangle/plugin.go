package tangle

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
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

	onSolidMilestoneChanged        *events.Closure
	onPruningMilestoneIndexChanged *events.Closure
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

	if err := address.ValidAddress(config.NodeConfig.GetString(config.CfgCoordinatorAddress)); err != nil {
		log.Fatal(err.Error())
	}

	tangle.ConfigureMilestones(
		hornet.Hash(trinary.MustTrytesToBytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress))[:49]),
		config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel),
		uint64(config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth)),
	)

	configureEvents()
	configureTangleProcessor(plugin)

	gossip.AddRequestBackpressureSignal(IsReceiveTxWorkerPoolBusy)
}

func run(plugin *node.Plugin) {

	if tangle.IsDatabaseCorrupted() && !config.NodeConfig.GetBool(config.CfgDatabaseDebug) {
		log.Warnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := revalidateDatabase(); err != nil {
			if err == tangle.ErrOperationAborted {
				log.Info("database revalidation aborted")
				os.Exit(0)
			}
			log.Panic(errors.Wrap(ErrDatabaseRevalidationFailed, err.Error()))
		}
		log.Info("database revalidation successful")
	}

	// run a full database garbage collection at startup
	database.RunGarbageCollection()

	daemon.BackgroundWorker("Tangle[HeartbeatEvents]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityHeartbeats)

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

	// set latest known milestone from database
	latestMilestoneFromDatabase := tangle.SearchLatestMilestoneIndexInStore()
	if latestMilestoneFromDatabase < tangle.GetSolidMilestoneIndex() {
		latestMilestoneFromDatabase = tangle.GetSolidMilestoneIndex()
	}
	tangle.SetLatestMilestoneIndex(latestMilestoneFromDatabase, updateSyncedAtStartup)

	runTangleProcessor(plugin)

	// create a background worker that prints a status message every second
	daemon.BackgroundWorker("Tangle status reporter", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityStatusReport)
}

func configureEvents() {
	onSolidMilestoneChanged = events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		defer cachedBndl.Release() // bundle -1
		// notify peers about our new solid milestone index
		gossip.BroadcastHeartbeat()
	})

	onPruningMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new pruning milestone index
		gossip.BroadcastHeartbeat()
	})
}

func attachEvents() {
	Events.SolidMilestoneChanged.Attach(onSolidMilestoneChanged)
	Events.PruningMilestoneIndexChanged.Attach(onPruningMilestoneIndexChanged)
}

func detachEvents() {
	Events.SolidMilestoneChanged.Detach(onSolidMilestoneChanged)
	Events.PruningMilestoneIndexChanged.Detach(onPruningMilestoneIndexChanged)
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func SetUpdateSyncedAtStartup(updateSynced bool) {
	updateSyncedAtStartup = updateSynced
}
