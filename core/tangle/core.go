package tangle

import (
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/pkg/keymanager"
	coopkg "github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/plugins/coordinator"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	flag.CommandLine.MarkHidden("syncedAtStartup")
	CoreModule = &node.CoreModule{
		Name:      "Tangle",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Configure: configure,
		Run:       run,
	}
}

var (
	CoreModule *node.CoreModule
	log        *logger.Logger
	deps       dependencies

	updateSyncedAtStartup bool
	syncedAtStartup       = flag.Bool("syncedAtStartup", false, "LMI is set to LSMI at startup")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new local snapshot.")

	onSolidMilestoneIndexChanged   *events.Closure
	onPruningMilestoneIndexChanged *events.Closure
	onLatestMilestoneIndexChanged  *events.Closure
	onReceivedNewTx                *events.Closure
)

type dependencies struct {
	dig.In
	Tangle                     *tangle.Tangle
	Manager                    *p2p.Manager
	RequestQueue               gossippkg.RequestQueue
	MessageProcessor           *gossippkg.MessageProcessor
	NodeConfig                 *configuration.Configuration `name:"nodeConfig"`
	CoordinatorPublicKeyRanges coordinator.PublicKeyRanges
}

func configure() {
	log = logger.NewLogger(CoreModule.Name)

	deps.Tangle.LoadInitialValuesFromDatabase()

	updateSyncedAtStartup = *syncedAtStartup

	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	CoreModule.Daemon().BackgroundWorker("Database Health", func(shutdownSignal <-chan struct{}) {
		deps.Tangle.MarkDatabaseCorrupted()
	})

	keyManager := keymanager.New()
	for _, keyRange := range deps.CoordinatorPublicKeyRanges {
		if err := keyManager.AddKeyRange(keyRange.Key, keyRange.StartIndex, keyRange.EndIndex); err != nil {
			log.Panicf("can't load public key ranges: %w", err)
		}
	}

	deps.Tangle.ConfigureMilestones(
		keyManager,
		deps.NodeConfig.Int(coordinator.CfgCoordinatorMilestonePublicKeyCount),
		coopkg.MilestoneMerkleTreeHashFuncWithName(deps.NodeConfig.String(coordinator.CfgCoordinatorMilestoneMerkleTreeHashFunc)),
	)

	configureEvents()
	configureTangleProcessor()

	gossip.AddRequestBackpressureSignal(IsReceiveTxWorkerPoolBusy)
}

func run() {

	if deps.Tangle.IsDatabaseCorrupted() && !deps.NodeConfig.Bool(database.CfgDatabaseDebug) {
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

	CoreModule.Daemon().BackgroundWorker("Tangle[SolidifierGossipEvents]", func(shutdownSignal <-chan struct{}) {
		attachSolidifierGossipEvents()
		<-shutdownSignal
		detachSolidifierGossipEvents()
	}, shutdown.PrioritySolidifierGossip)

	CoreModule.Daemon().BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		abortMilestoneSolidification()

		log.Info("Flushing caches to database...")
		deps.Tangle.ShutdownStorages()
		log.Info("Flushing caches to database... done")

	}, shutdown.PriorityFlushToDatabase)

	// set latest known milestone from database
	latestMilestoneFromDatabase := deps.Tangle.SearchLatestMilestoneIndexInStore()
	if latestMilestoneFromDatabase < deps.Tangle.GetSolidMilestoneIndex() {
		latestMilestoneFromDatabase = deps.Tangle.GetSolidMilestoneIndex()
	}
	deps.Tangle.SetLatestMilestoneIndex(latestMilestoneFromDatabase, updateSyncedAtStartup)

	runTangleProcessor()

	// create a background worker that prints a status message every second
	CoreModule.Daemon().BackgroundWorker("Tangle status reporter", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityStatusReport)

	// create a background worker that "measures" the MPS value every second
	CoreModule.Daemon().BackgroundWorker("Metrics MPS Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(measureMPS, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityMetricsUpdater)
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

		if deps.Tangle.IsNodeSyncedWithThreshold() {
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
