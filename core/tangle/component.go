package tangle

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/timeutil"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/migrator"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

const (
	// CfgTangleSyncedAtStartup defines the LMI is set to CMI at startup.
	CfgTangleSyncedAtStartup = "syncedAtStartup"
	// CfgTangleRevalidateDatabase defines whether to revalidate the database on startup if corrupted.
	CfgTangleRevalidateDatabase = "revalidate"
)

func init() {
	_ = flag.CommandLine.MarkHidden(CfgTangleSyncedAtStartup)

	Component = &app.Component{
		Name:      "Tangle",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Params:    params,
		IsEnabled: components.IsAutopeeringEntryNodeDisabled, // do not enable in "autopeering entry node" mode
		Provide:   provide,
		Configure: configure,
		Run:       run,
	}
}

var (
	Component *app.Component
	deps      dependencies

	syncedAtStartup    = flag.Bool(CfgTangleSyncedAtStartup, false, "LMI is set to CMI at startup")
	revalidateDatabase = flag.Bool(CfgTangleRevalidateDatabase, false, "revalidate the database on startup if corrupted")

	// ErrDatabaseRevalidationFailed is return when the database revalidation failed.
	//
	//nolint:revive // this error message is shown to the user
	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new snapshot.")
)

type dependencies struct {
	dig.In
	Storage                  *storage.Storage
	Tangle                   *tangle.Tangle
	Requester                *gossip.Requester
	Broadcaster              *gossip.Broadcaster
	SnapshotImporter         *snapshot.Importer
	PruningManager           *pruning.Manager
	DatabaseDebug            bool `name:"databaseDebug"`
	DatabaseAutoRevalidation bool `name:"databaseAutoRevalidation"`
	PruneReceipts            bool `name:"pruneReceipts"`
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() *metrics.ServerMetrics {
		return &metrics.ServerMetrics{}
	}); err != nil {
		Component.LogPanic(err)
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
		Component.LogPanic(err)
	}

	type cfgResult struct {
		dig.Out
		MaxDeltaBlockYoungestConeRootIndexToCMI int `name:"maxDeltaBlockYoungestConeRootIndexToCMI"`
		MaxDeltaBlockOldestConeRootIndexToCMI   int `name:"maxDeltaBlockOldestConeRootIndexToCMI"`
	}

	if err := c.Provide(func() cfgResult {
		return cfgResult{
			MaxDeltaBlockYoungestConeRootIndexToCMI: ParamsTangle.MaxDeltaBlockYoungestConeRootIndexToCMI,
			MaxDeltaBlockOldestConeRootIndexToCMI:   ParamsTangle.MaxDeltaBlockOldestConeRootIndexToCMI,
		}
	}); err != nil {
		Component.LogPanic(err)
	}

	type tipScoreDeps struct {
		dig.In
		Storage                                 *storage.Storage
		MaxDeltaBlockYoungestConeRootIndexToCMI int `name:"maxDeltaBlockYoungestConeRootIndexToCMI"`
		MaxDeltaBlockOldestConeRootIndexToCMI   int `name:"maxDeltaBlockOldestConeRootIndexToCMI"`
		ProtocolManager                         *protocol.Manager
	}

	if err := c.Provide(func(deps tipScoreDeps) *tangle.TipScoreCalculator {
		return tangle.NewTipScoreCalculator(deps.Storage, deps.MaxDeltaBlockYoungestConeRootIndexToCMI, deps.MaxDeltaBlockOldestConeRootIndexToCMI, int(deps.ProtocolManager.Current().BelowMaxDepth))
	}); err != nil {
		Component.LogPanic(err)
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
		ReceiptService   *migrator.ReceiptService `optional:"true"`
		ProtocolManager  *protocol.Manager
	}

	if err := c.Provide(func(deps tangleDeps) *tangle.Tangle {
		return tangle.New(
			Component.Daemon().ContextStopped(),
			Component.Daemon(),
			Component.App().NewLogger("Tangle"),
			deps.Storage,
			deps.SyncManager,
			deps.MilestoneManager,
			deps.RequestQueue,
			deps.Service,
			deps.MessageProcessor,
			deps.ServerMetrics,
			deps.Requester,
			deps.ReceiptService,
			deps.ProtocolManager,
			ParamsTangle.MilestoneTimeout,
			ParamsTangle.WhiteFlagParentsSolidTimeout,
			*syncedAtStartup)
	}); err != nil {
		Component.LogPanic(err)
	}

	return nil
}

func configure() error {
	// Create a background worker that marks the database as corrupted at clean startup.
	// This has to be done in a background worker, because the Daemon could receive
	// a shutdown signal during startup. If that is the case, the BackgroundWorker will never be started
	// and the database will never be marked as corrupted.
	if err := Component.Daemon().BackgroundWorker("Database Health", func(_ context.Context) {
		if err := deps.Storage.MarkStoresCorrupted(); err != nil {
			Component.LogPanic(err)
		}
	}, daemon.PriorityDatabaseHealth); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	databaseCorrupted, err := deps.Storage.AreStoresCorrupted()
	if err != nil {
		Component.LogPanic(err)
	}

	if databaseCorrupted && !deps.DatabaseDebug {
		// no need to check for the "deleteDatabase" and "deleteAll" flags,
		// since the database should only be marked as corrupted,
		// if it was not deleted before this check.
		revalidateDatabase := *revalidateDatabase || deps.DatabaseAutoRevalidation
		if !revalidateDatabase {
			Component.LogPanic(`
HORNET was not shut down properly, the database may be corrupted.
Please restart HORNET with one of the following flags or enable "db.autoRevalidation" in the config.

--revalidate:     starts the database revalidation (might take a long time)
--deleteDatabase: deletes the database
--deleteAll:      deletes the database and the snapshot files
`)
		}
		Component.LogWarnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation ...")

		if err := deps.Tangle.RevalidateDatabase(deps.SnapshotImporter, deps.PruneReceipts); err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				Component.LogInfo("database revalidation aborted")
				os.Exit(0)
			}
			Component.LogPanicf("%s: %s", ErrDatabaseRevalidationFailed, err)
		}
		Component.LogInfo("database revalidation successful")
	}

	deps.Tangle.ConfigureTangleProcessor()

	return nil
}

func run() error {
	if err := Component.Daemon().BackgroundWorker("Tangle[HeartbeatEvents]", func(ctx context.Context) {
		unhook := attachHeartbeatEvents()
		defer unhook()
		<-ctx.Done()
	}, daemon.PriorityHeartbeats); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	if err := Component.Daemon().BackgroundWorker("Cleanup at shutdown", func(ctx context.Context) {
		<-ctx.Done()
		deps.Tangle.AbortMilestoneSolidification()

		Component.LogInfo("Flushing caches to database ...")
		deps.Storage.ShutdownStorages()
		Component.LogInfo("Flushing caches to database ... done")

	}, daemon.PriorityFlushToDatabase); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	deps.Tangle.RunTangleProcessor()

	// create a background worker that prints a status message every second
	if err := Component.Daemon().BackgroundWorker("Tangle status reporter", func(ctx context.Context) {
		ticker := timeutil.NewTicker(deps.Tangle.PrintStatus, 1*time.Second, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PriorityStatusReport); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func attachHeartbeatEvents() (detach func()) {
	return lo.Batch(
		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Hook(func(_ iotago.MilestoneIndex) {
			// notify peers about our new solid milestone index
			// bee differentiates between solid and confirmed milestone, for hornet it is the same.
			deps.Broadcaster.BroadcastHeartbeat(nil)
		}).Unhook,

		deps.PruningManager.Events.PruningMilestoneIndexChanged.Hook(func(_ iotago.MilestoneIndex) {
			// notify peers about our new pruning milestone index
			deps.Broadcaster.BroadcastHeartbeat(nil)
		}).Unhook,

		deps.Tangle.Events.LatestMilestoneIndexChanged.Hook(func(_ iotago.MilestoneIndex) {
			// notify peers about our new latest milestone index
			deps.Broadcaster.BroadcastHeartbeat(nil)
		}).Unhook,
	)
}
