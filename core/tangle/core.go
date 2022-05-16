package tangle

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// LMI is set to CMI at startup
	CfgTangleSyncedAtStartup = "syncedAtStartup"
	// whether to revalidate the database on startup if corrupted
	CfgTangleRevalidateDatabase = "revalidate"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgTangleSyncedAtStartup)

	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
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
	CoreComponent *app.CoreComponent
	deps          dependencies

	syncedAtStartup    = flag.Bool(CfgTangleSyncedAtStartup, false, "LMI is set to CMI at startup")
	revalidateDatabase = flag.Bool(CfgTangleRevalidateDatabase, false, "revalidate the database on startup if corrupted")

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new snapshot.")

	onConfirmedMilestoneIndexChanged *events.Closure
	onPruningMilestoneIndexChanged   *events.Closure
	onLatestMilestoneIndexChanged    *events.Closure
)

type dependencies struct {
	dig.In
	Storage                  *storage.Storage
	Tangle                   *tangle.Tangle
	Requester                *gossip.Requester
	Broadcaster              *gossip.Broadcaster
	SnapshotManager          *snapshot.SnapshotManager
	DatabaseDebug            bool `name:"databaseDebug"`
	DatabaseAutoRevalidation bool `name:"databaseAutoRevalidation"`
	PruneReceipts            bool `name:"pruneReceipts"`
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() *metrics.ServerMetrics {
		return &metrics.ServerMetrics{}
	}); err != nil {
		CoreComponent.LogPanic(err)
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
		CoreComponent.LogPanic(err)
	}

	type cfgResult struct {
		dig.Out
		MaxDeltaMsgYoungestConeRootIndexToCMI int `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
		MaxDeltaMsgOldestConeRootIndexToCMI   int `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			MaxDeltaMsgYoungestConeRootIndexToCMI: ParamsTangle.MaxDeltaMsgYoungestConeRootIndexToCMI,
			MaxDeltaMsgOldestConeRootIndexToCMI:   ParamsTangle.MaxDeltaMsgOldestConeRootIndexToCMI,
		}
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	type tipScoreDeps struct {
		dig.In
		Storage                               *storage.Storage
		MaxDeltaMsgYoungestConeRootIndexToCMI int `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
		MaxDeltaMsgOldestConeRootIndexToCMI   int `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
		ProtocolParameters                    *iotago.ProtocolParameters
	}

	if err := c.Provide(func(deps tipScoreDeps) *tangle.TipScoreCalculator {
		return tangle.NewTipScoreCalculator(deps.Storage, deps.MaxDeltaMsgYoungestConeRootIndexToCMI, deps.MaxDeltaMsgOldestConeRootIndexToCMI, int(deps.ProtocolParameters.BelowMaxDepth))
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	type tangleDeps struct {
		dig.In
		Storage            *storage.Storage
		SyncManager        *syncmanager.SyncManager
		MilestoneManager   *milestonemanager.MilestoneManager
		RequestQueue       gossip.RequestQueue
		Service            *gossip.Service
		Requester          *gossip.Requester
		MessageProcessor   *gossip.MessageProcessor
		ServerMetrics      *metrics.ServerMetrics
		ReceiptService     *migrator.ReceiptService `optional:"true"`
		ProtocolParameters *iotago.ProtocolParameters
	}

	if err := c.Provide(func(deps tangleDeps) *tangle.Tangle {
		return tangle.New(
			logger.NewLogger("Tangle"),
			CoreComponent.Daemon(),
			CoreComponent.Daemon().ContextStopped(),
			deps.Storage,
			deps.SyncManager,
			deps.MilestoneManager,
			deps.RequestQueue,
			deps.Service,
			deps.MessageProcessor,
			deps.ServerMetrics,
			deps.Requester,
			deps.ReceiptService,
			deps.ProtocolParameters,
			ParamsTangle.MilestoneTimeout,
			ParamsTangle.WhiteFlagParentsSolidTimeout,
			*syncedAtStartup)
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	return nil
}

func configure() error {
	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	if err := CoreComponent.Daemon().BackgroundWorker("Database Health", func(_ context.Context) {
		if err := deps.Storage.MarkDatabasesCorrupted(); err != nil {
			CoreComponent.LogPanic(err)
		}
	}, daemon.PriorityDatabaseHealth); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	databaseCorrupted, err := deps.Storage.AreDatabasesCorrupted()
	if err != nil {
		CoreComponent.LogPanic(err)
	}

	if databaseCorrupted && !deps.DatabaseDebug {
		// no need to check for the "deleteDatabase" and "deleteAll" flags,
		// since the database should only be marked as corrupted,
		// if it was not deleted before this check.
		revalidateDatabase := *revalidateDatabase || deps.DatabaseAutoRevalidation
		if !revalidateDatabase {
			CoreComponent.LogPanic(`
HORNET was not shut down properly, the database may be corrupted.
Please restart HORNET with one of the following flags or enable "db.autoRevalidation" in the config.

--revalidate:     starts the database revalidation (might take a long time)
--deleteDatabase: deletes the database
--deleteAll:      deletes the database and the snapshot files
`)
		}
		CoreComponent.LogWarnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := deps.Tangle.RevalidateDatabase(deps.SnapshotManager, deps.PruneReceipts); err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				CoreComponent.LogInfo("database revalidation aborted")
				os.Exit(0)
			}
			CoreComponent.LogPanicf("%s: %s", ErrDatabaseRevalidationFailed, err)
		}
		CoreComponent.LogInfo("database revalidation successful")
	}

	configureEvents()
	deps.Tangle.ConfigureTangleProcessor()

	return nil
}

func run() error {

	if err := CoreComponent.Daemon().BackgroundWorker("Tangle[HeartbeatEvents]", func(ctx context.Context) {
		attachHeartbeatEvents()
		<-ctx.Done()
		detachHeartbeatEvents()
	}, daemon.PriorityHeartbeats); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	if err := CoreComponent.Daemon().BackgroundWorker("Cleanup at shutdown", func(ctx context.Context) {
		<-ctx.Done()
		deps.Tangle.AbortMilestoneSolidification()

		CoreComponent.LogInfo("Flushing caches to database...")
		deps.Storage.ShutdownStorages()
		CoreComponent.LogInfo("Flushing caches to database... done")

	}, daemon.PriorityFlushToDatabase); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	deps.Tangle.RunTangleProcessor()

	// create a background worker that prints a status message every second
	if err := CoreComponent.Daemon().BackgroundWorker("Tangle status reporter", func(ctx context.Context) {
		ticker := timeutil.NewTicker(deps.Tangle.PrintStatus, 1*time.Second, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PriorityStatusReport); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	return nil
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
