package tangle

import (
	"os"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/database"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN                = node.NewPlugin("Tangle", node.Enabled, configure, run)
	log                   *logger.Logger
	updateSyncedAtStartup bool

	syncedAtStartup = flag.Bool("syncedAtStartup", false, "LMI is set to LSMI at startup")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new local snapshot.")

	onSolidMilestoneIndexChanged   *events.Closure
	onPruningMilestoneIndexChanged *events.Closure
	onLatestMilestoneIndexChanged  *events.Closure
	onReceivedNewTx                *events.Closure
)

func init() {
	flag.CommandLine.MarkHidden("syncedAtStartup")
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	tangle.LoadInitialValuesFromDatabase()

	updateSyncedAtStartup = *syncedAtStartup

	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	daemon.BackgroundWorker("Database Health", func(shutdownSignal <-chan struct{}) {
		tangle.MarkDatabaseCorrupted()
	})

	cooPublicKey, err := utils.ParseEd25519PublicKeyFromString(config.NodeConfig.GetString(config.CfgCoordinatorPublicKey))
	if err != nil {
		log.Fatal(err.Error())
	}

	tangle.ConfigureMilestones(
		cooPublicKey,
		coordinator.MilestoneMerkleTreeHashFuncWithName(config.NodeConfig.GetString(config.CfgCoordinatorMilestoneMerkleTreeHashFunc)),
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

	attachHeartbeatEvents()
	detachHeartbeatEvents()

	daemon.BackgroundWorker("Tangle[SolidifierGossipEvents]", func(shutdownSignal <-chan struct{}) {
		attachSolidifierGossipEvents()
		<-shutdownSignal
		detachSolidifierGossipEvents()
	}, shutdown.PrioritySolidifierGossip)

	daemon.BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		abortMilestoneSolidification()

		log.Info("Flushing caches to database...")
		tangle.ShutdownStorages()
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
		//timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityStatusReport)
}

func configureEvents() {
	onSolidMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new solid milestone index
		gossip.BroadcastHeartbeat(nil)
	})

	onPruningMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new pruning milestone index
		gossip.BroadcastHeartbeat(nil)
	})

	onLatestMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new latest milestone index
		gossip.BroadcastHeartbeat(nil)
	})

	onReceivedNewTx = events.NewClosure(func(cachedMsg *tangle.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		// Force release possible here, since processIncomingTx still holds a reference
		defer cachedMsg.Release(true) // msg -1

		if tangle.IsNodeSyncedWithThreshold() {
			solidifyFutureConeOfMsg(cachedMsg.GetCachedMetadata()) // meta pass +1
		}
	})
}

func attachHeartbeatEvents() {
	Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
	Events.PruningMilestoneIndexChanged.Attach(onPruningMilestoneIndexChanged)
	Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
}

func attachSolidifierGossipEvents() {
	Events.ReceivedNewMessage.Attach(onReceivedNewTx)
}

func detachHeartbeatEvents() {
	Events.SolidMilestoneIndexChanged.Detach(onSolidMilestoneIndexChanged)
	Events.PruningMilestoneIndexChanged.Detach(onPruningMilestoneIndexChanged)
	Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
}

func detachSolidifierGossipEvents() {
	Events.ReceivedNewMessage.Detach(onReceivedNewTx)
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func SetUpdateSyncedAtStartup(updateSynced bool) {
	updateSyncedAtStartup = updateSynced
}
