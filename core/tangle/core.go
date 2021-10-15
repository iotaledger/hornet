package tangle

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
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
	_ = flag.CommandLine.MarkHidden(CfgTangleSyncedAtStartup)

	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Tangle",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
	deps       dependencies

	syncedAtStartup    = flag.Bool(CfgTangleSyncedAtStartup, false, "LMI is set to CMI at startup")
	revalidateDatabase = flag.Bool(CfgTangleRevalidateDatabase, false, "revalidate the database on startup if corrupted")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new snapshot.")

	onConfirmedMilestoneIndexChanged *events.Closure
	onPruningMilestoneIndexChanged   *events.Closure
	onLatestMilestoneIndexChanged    *events.Closure
)

type dependencies struct {
	dig.In
	TangleDatabase           *database.Database `name:"tangleDatabase"`
	Storage                  *storage.Storage
	Tangle                   *tangle.Tangle
	Requester                *gossip.Requester
	Broadcaster              *gossip.Broadcaster
	SnapshotManager          *snapshot.SnapshotManager
	NodeConfig               *configuration.Configuration `name:"nodeConfig"`
	DatabaseDebug            bool                         `name:"databaseDebug"`
	DatabaseAutoRevalidation bool                         `name:"databaseAutoRevalidation"`
	PruneReceipts            bool                         `name:"pruneReceipts"`
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *metrics.ServerMetrics {
		return &metrics.ServerMetrics{}
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type milestoneManagerDeps struct {
		dig.In
		Storage                 *storage.Storage
		SyncManager             *syncmanager.SyncManager
		CoordinatorKeyManager   *keymanager.KeyManager
		MilestonePublicKeyCount int `name:"milestonePublicKeyCount"`
	}

	if err := c.Provide(func(deps milestoneManagerDeps) *milestonemanager.MilestoneManager {
		return milestonemanager.New(
			deps.Storage,
			deps.SyncManager,
			deps.CoordinatorKeyManager,
			deps.MilestonePublicKeyCount)
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type tangleDeps struct {
		dig.In
		Storage          *storage.Storage
		SyncManager      *syncmanager.SyncManager
		MilestoneManager *milestonemanager.MilestoneManager
		RequestQueue     gossip.RequestQueue
		Service          *gossip.Service
		Requester        *gossip.Requester
		MessageProcessor *gossip.MessageProcessor
		ServerMetrics    *metrics.ServerMetrics
		ReceiptService   *migrator.ReceiptService     `optional:"true"`
		NodeConfig       *configuration.Configuration `name:"nodeConfig"`
		BelowMaxDepth    int                          `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps tangleDeps) *tangle.Tangle {
		return tangle.New(
			logger.NewLogger("Tangle"),
			CorePlugin.Daemon(),
			CorePlugin.Daemon().ContextStopped(),
			deps.Storage,
			deps.SyncManager,
			deps.MilestoneManager,
			deps.RequestQueue,
			deps.Service,
			deps.MessageProcessor,
			deps.ServerMetrics,
			deps.Requester,
			deps.ReceiptService,
			deps.BelowMaxDepth,
			deps.NodeConfig.Duration(CfgTangleMilestoneTimeout),
			*syncedAtStartup)
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {
	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	if err := CorePlugin.Daemon().BackgroundWorker("Database Health", func(_ context.Context) {
		if err := deps.Storage.MarkDatabasesCorrupted(); err != nil {
			CorePlugin.Panic(err)
		}
	}, shutdown.PriorityDatabaseHealth); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	databaseCorrupted, err := deps.Storage.AreDatabasesCorrupted()
	if err != nil {
		CorePlugin.Panic(err)
	}

	if databaseCorrupted && !deps.DatabaseDebug {
		// no need to check for the "deleteDatabase" and "deleteAll" flags,
		// since the database should only be marked as corrupted,
		// if it was not deleted before this check.
		revalidateDatabase := *revalidateDatabase || deps.DatabaseAutoRevalidation
		if !revalidateDatabase {
			CorePlugin.Panic(`
HORNET was not shut down properly, the database may be corrupted.
Please restart HORNET with one of the following flags or enable "db.autoRevalidation" in the config.

--revalidate:     starts the database revalidation (might take a long time)
--deleteDatabase: deletes the database
--deleteAll:      deletes the database and the snapshot files
`)
		}
		CorePlugin.LogWarnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := deps.Tangle.RevalidateDatabase(deps.SnapshotManager, deps.PruneReceipts); err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				CorePlugin.LogInfo("database revalidation aborted")
				os.Exit(0)
			}
			CorePlugin.Panicf("%s: %s", ErrDatabaseRevalidationFailed, err)
		}
		CorePlugin.LogInfo("database revalidation successful")
	}

	configureEvents()
	deps.Tangle.ConfigureTangleProcessor()
}

func run() {

	// run a full database garbage collection at startup
	deps.TangleDatabase.RunGarbageCollection()
	//TODO: run it for the UTXODatabase? If yes, where?

	if err := CorePlugin.Daemon().BackgroundWorker("Tangle[HeartbeatEvents]", func(ctx context.Context) {
		attachHeartbeatEvents()
		<-ctx.Done()
		detachHeartbeatEvents()
	}, shutdown.PriorityHeartbeats); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("Cleanup at shutdown", func(ctx context.Context) {
		<-ctx.Done()
		deps.Tangle.AbortMilestoneSolidification()

		CorePlugin.LogInfo("Flushing caches to database...")
		deps.Storage.ShutdownStorages()
		CorePlugin.LogInfo("Flushing caches to database... done")

	}, shutdown.PriorityFlushToDatabase); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	deps.Tangle.RunTangleProcessor()

	// create a background worker that prints a status message every second
	if err := CorePlugin.Daemon().BackgroundWorker("Tangle status reporter", func(ctx context.Context) {
		ticker := timeutil.NewTicker(deps.Tangle.PrintStatus, 1*time.Second, ctx)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityStatusReport); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

}

func configureEvents() {
	onConfirmedMilestoneIndexChanged = events.NewClosure(func(_ milestone.Index) {
		// notify peers about our new solid milestone index
		// bee differentiates between solid and confirmed milestone, for hornet it is the same.
		deps.Broadcaster.BroadcastHeartbeat(nil)
	})

	onPruningMilestoneIndexChanged = events.NewClosure(func(_ milestone.Index) {
		// notify peers about our new pruning milestone index
		deps.Broadcaster.BroadcastHeartbeat(nil)
	})

	onLatestMilestoneIndexChanged = events.NewClosure(func(_ milestone.Index) {
		// notify peers about our new latest milestone index
		deps.Broadcaster.BroadcastHeartbeat(nil)
	})
}

func attachHeartbeatEvents() {
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
	deps.SnapshotManager.Events.PruningMilestoneIndexChanged.Attach(onPruningMilestoneIndexChanged)
	deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
}

func detachHeartbeatEvents() {
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
	deps.SnapshotManager.Events.PruningMilestoneIndexChanged.Detach(onPruningMilestoneIndexChanged)
	deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
}
