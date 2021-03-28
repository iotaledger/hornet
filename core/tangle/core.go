package tangle

import (
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	snapshotpkg "github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
)

const (
	// LMI is set to CMI at startup
	CfgTangleSyncedAtStartup = "syncedAtStartup"
	// whether to revalidate the database on startup if corrupted
	CfgTangleRevalidateDatabase = "revalidate"
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

	syncedAtStartup    = flag.Bool(CfgTangleSyncedAtStartup, false, "LMI is set to CMI at startup")
	revalidateDatabase = flag.Bool(CfgTangleRevalidateDatabase, false, "revalidate the database on startup if corrupted")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new snapshot.")

	onConfirmedMilestoneIndexChanged *events.Closure
	onPruningMilestoneIndexChanged   *events.Closure
	onLatestMilestoneIndexChanged    *events.Closure
	onReceivedNewTx                  *events.Closure
)

type dependencies struct {
	dig.In
	Storage     *storage.Storage
	Tangle      *tangle.Tangle
	Requester   *gossip.Requester
	Broadcaster *gossip.Broadcaster
	Snapshot    *snapshotpkg.Snapshot
	NodeConfig  *configuration.Configuration `name:"nodeConfig"`
}

func provide(c *dig.Container) {
	if err := c.Provide(func() *metrics.ServerMetrics {
		return &metrics.ServerMetrics{}
	}); err != nil {
		panic(err)
	}

	type belowmaxdepthdeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps belowmaxdepthdeps) int {
		return deps.NodeConfig.Int(urts.CfgTipSelBelowMaxDepth)
	}, dig.Name("belowMaxDepth")); err != nil {
		panic(err)
	}

	type tangledeps struct {
		dig.In
		Storage          *storage.Storage
		RequestQueue     gossippkg.RequestQueue
		Service          *gossippkg.Service
		Requester        *gossippkg.Requester
		MessageProcessor *gossippkg.MessageProcessor
		ServerMetrics    *metrics.ServerMetrics
		ReceiptService   *migrator.ReceiptService `optional:"true"`
		BelowMaxDepth    int                      `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps tangledeps) *tangle.Tangle {
		return tangle.New(
			logger.NewLogger("Tangle"),
			deps.Storage,
			deps.RequestQueue,
			deps.Service,
			deps.MessageProcessor,
			deps.ServerMetrics,
			deps.Requester,
			deps.ReceiptService,
			CorePlugin.Daemon(),
			CorePlugin.Daemon().ContextStopped(),
			deps.BelowMaxDepth,
			*syncedAtStartup)
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(CorePlugin.Name)

	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	CorePlugin.Daemon().BackgroundWorker("Database Health", func(shutdownSignal <-chan struct{}) {
		deps.Storage.MarkDatabaseCorrupted()
	})

	if deps.Storage.IsDatabaseCorrupted() && !deps.NodeConfig.Bool(database.CfgDatabaseDebug) {
		// no need to check for the "deleteDatabase" and "deleteAll" flags,
		// since the database should only be marked as corrupted,
		// if it was not deleted before this check.
		revalidateDatabase := *revalidateDatabase || deps.NodeConfig.Bool(database.CfgDatabaseAutoRevalidation)
		if !revalidateDatabase {
			log.Panic(`
HORNET was not shut down properly, the database may be corrupted.
Please restart HORNET with one of the following flags or enable "db.autoRevalidation" in the config.

--revalidate:     starts the database revalidation (might take a long time)
--deleteDatabase: deletes the database
--deleteAll:      deletes the database and the snapshot files
`)
		}
		log.Warnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := deps.Tangle.RevalidateDatabase(deps.Snapshot, deps.NodeConfig.Bool(snapshot.CfgPruningPruneReceipts)); err != nil {
			if err == common.ErrOperationAborted {
				log.Info("database revalidation aborted")
				os.Exit(0)
			}
			log.Panicf("%s %s", ErrDatabaseRevalidationFailed, err)
		}
		log.Info("database revalidation successful")
	}

	configureEvents()
	deps.Tangle.ConfigureTangleProcessor()
}

func run() {

	// run a full database garbage collection at startup
	database.RunGarbageCollection()

	_ = CorePlugin.Daemon().BackgroundWorker("Tangle[SolidifierGossipEvents]", func(shutdownSignal <-chan struct{}) {
		attachSolidifierGossipEvents()
		attachHeartbeatEvents()
		<-shutdownSignal
		detachSolidifierGossipEvents()
		detachHeartbeatEvents()
	}, shutdown.PrioritySolidifierGossip)

	_ = CorePlugin.Daemon().BackgroundWorker("Cleanup at shutdown", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		deps.Tangle.AbortMilestoneSolidification()

		log.Info("Flushing caches to database...")
		deps.Storage.ShutdownStorages()
		log.Info("Flushing caches to database... done")

	}, shutdown.PriorityFlushToDatabase)

	deps.Tangle.RunTangleProcessor()

	// create a background worker that prints a status message every second
	_ = CorePlugin.Daemon().BackgroundWorker("Tangle status reporter", func(shutdownSignal <-chan struct{}) {
		ticker := timeutil.NewTicker(deps.Tangle.PrintStatus, 1*time.Second, shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityStatusReport)

}

func configureEvents() {
	onConfirmedMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new solid milestone index
		// bee differentiates between solid and confirmed milestone, for hornet it is the same.
		deps.Broadcaster.BroadcastHeartbeat(nil)
	})

	onPruningMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new pruning milestone index
		deps.Broadcaster.BroadcastHeartbeat(nil)
	})

	onLatestMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new latest milestone index
		deps.Broadcaster.BroadcastHeartbeat(nil)
	})

	onReceivedNewTx = events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		// Force release possible here, since processIncomingTx still holds a reference
		defer cachedMsg.Release(true) // msg -1

		if deps.Storage.IsNodeAlmostSynced() {
			deps.Tangle.SolidifyFutureConeOfMsg(cachedMsg.GetCachedMetadata()) // meta pass +1
		}
	})
}

func attachHeartbeatEvents() {
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
	deps.Snapshot.Events.PruningMilestoneIndexChanged.Attach(onPruningMilestoneIndexChanged)
	deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
}

func attachSolidifierGossipEvents() {
	deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewTx)
}

func detachHeartbeatEvents() {
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
	deps.Snapshot.Events.PruningMilestoneIndexChanged.Detach(onPruningMilestoneIndexChanged)
	deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
}

func detachSolidifierGossipEvents() {
	deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewTx)
}
