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
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
)

const (
	// LMI is set to LSMI at startup
	CfgTangleSyncedAtStartup = "syncedAtStartup"
)

func init() {
	flag.CommandLine.MarkHidden(CfgTangleSyncedAtStartup)

	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Tangle",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	log        *logger.Logger
	deps       dependencies

	syncedAtStartup = flag.Bool(CfgTangleSyncedAtStartup, false, "LMI is set to LSMI at startup")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new local snapshot.")

	onSolidMilestoneIndexChanged   *events.Closure
	onPruningMilestoneIndexChanged *events.Closure
	onLatestMilestoneIndexChanged  *events.Closure
	onReceivedNewTx                *events.Closure
)

type dependencies struct {
	dig.In
	Storage *storage.Storage
	ServerMetrics              *metrics.ServerMetrics
	Manager                    *p2p.Manager
	RequestQueue               gossippkg.RequestQueue
	MessageProcessor           *gossippkg.MessageProcessor
	Service                    *gossippkg.Service
	NodeConfig                 *configuration.Configuration `name:"nodeConfig"`
	CoordinatorPublicKeyRanges coordinator.PublicKeyRanges
}

func provide(c *dig.Container) {
	if err := c.Provide(func() *metrics.ServerMetrics {
		return &metrics.ServerMetrics{}
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CorePlugin.Name)

	deps.Storage.LoadInitialValuesFromDatabase()

	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	CorePlugin.Daemon().BackgroundWorker("Database Health", func(shutdownSignal <-chan struct{}) {
		deps.Storage.MarkDatabaseCorrupted()
	})

	keyManager := keymanager.New()
	for _, keyRange := range deps.CoordinatorPublicKeyRanges {
		if err := keyManager.AddKeyRange(keyRange.Key, keyRange.StartIndex, keyRange.EndIndex); err != nil {
			log.Panicf("can't load public key ranges: %s", err)
		}
	}

	deps.Storage.ConfigureMilestones(
		keyManager,
		deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount),
		coordinator.MilestoneMerkleTreeHashFuncWithName(deps.NodeConfig.String(protocfg.CfgProtocolMilestoneMerkleTreeHashFunc)),
	)

	configureEvents()
	configureTangleProcessor()

	gossip.AddRequestBackpressureSignal(IsReceiveTxWorkerPoolBusy)
}

func run() {

	if deps.Storage.IsDatabaseCorrupted() && !deps.NodeConfig.Bool(database.CfgDatabaseDebug) {
		log.Warnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := revalidateDatabase(); err != nil {
			if err == tangle.ErrOperationAborted {
				log.Info("database revalidation aborted")
				os.Exit(0)
			}
			log.Panicf("%s %s", ErrDatabaseRevalidationFailed, err)
		}
		log.Info("database revalidation successful")
	}

	// run a full database garbage collection at startup
	database.RunGarbageCollection()

	attachHeartbeatEvents()
	detachHeartbeatEvents()

	CorePlugin.Daemon().BackgroundWorker("Tangle[SolidifierGossipEvents]", func(shutdownSignal <-chan struct{}) {
		attachSolidifierGossipEvents()
		<-shutdownSignal
		detachSolidifierGossipEvents()
	}, shutdown.PrioritySolidifierGossip)

	CorePlugin.Daemon().BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		abortMilestoneSolidification()

		log.Info("Flushing caches to database...")
		deps.Storage.ShutdownStorages()
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
	CorePlugin.Daemon().BackgroundWorker("Tangle status reporter", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityStatusReport)

	// create a background worker that "measures" the MPS value every second
	CorePlugin.Daemon().BackgroundWorker("Metrics MPS Updater", func(shutdownSignal <-chan struct{}) {
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

	onReceivedNewTx = events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		// Force release possible here, since processIncomingTx still holds a reference
		defer cachedMsg.Release(true) // msg -1

		if deps.Storage.IsNodeSyncedWithThreshold() {
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
