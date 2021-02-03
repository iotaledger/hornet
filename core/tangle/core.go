package tangle

import (
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	gossippkg "github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
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

	ErrDatabaseRevalidationFailed = errors.New("Database revalidation failed! Please delete the database folder and start with a new snapshot.")

	onSolidMilestoneIndexChanged   *events.Closure
	onPruningMilestoneIndexChanged *events.Closure
	onLatestMilestoneIndexChanged  *events.Closure
	onReceivedNewTx                *events.Closure
)

type dependencies struct {
	dig.In
	Storage                    *storage.Storage
	Tangle                     *tangle.Tangle
	Requester                  *gossip.Requester
	Broadcaster                *gossip.Broadcaster
	NodeConfig                 *configuration.Configuration `name:"nodeConfig"`
	CoordinatorPublicKeyRanges coordinator.PublicKeyRanges
}

func provide(c *dig.Container) {
	if err := c.Provide(func() *metrics.ServerMetrics {
		return &metrics.ServerMetrics{}
	}); err != nil {
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
	}

	if err := c.Provide(func(deps tangledeps) *tangle.Tangle {
		return tangle.New(logger.NewLogger("Tangle"), deps.Storage, deps.RequestQueue, deps.Service, deps.MessageProcessor,
			deps.ServerMetrics, CorePlugin.Daemon().ContextStopped(), deps.Requester, CorePlugin.Daemon(), deps.ReceiptService, *syncedAtStartup)
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

	keyManager := keymanager.New()
	for _, keyRange := range deps.CoordinatorPublicKeyRanges {
		pubKey, err := utils.ParseEd25519PublicKeyFromString(keyRange.Key)
		if err != nil {
			log.Panicf("can't load public key ranges: %s", err)
		}

		keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
	}

	deps.Storage.ConfigureMilestones(
		keyManager,
		deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount),
	)

	configureEvents()
	deps.Tangle.ConfigureTangleProcessor()
}

func run() {

	if deps.Storage.IsDatabaseCorrupted() && !deps.NodeConfig.Bool(database.CfgDatabaseDebug) {
		log.Warnf("HORNET was not shut down correctly, the database may be corrupted. Starting revalidation...")

		if err := deps.Tangle.RevalidateDatabase(); err != nil {
			if err == common.ErrOperationAborted {
				log.Info("database revalidation aborted")
				os.Exit(0)
			}
			log.Panicf("%s %s", ErrDatabaseRevalidationFailed, err)
		}
		log.Info("database revalidation successful")
	}

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
		timeutil.NewTicker(deps.Tangle.PrintStatus, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityStatusReport)

}

func configureEvents() {
	onSolidMilestoneIndexChanged = events.NewClosure(func(msIndex milestone.Index) {
		// notify peers about our new solid milestone index
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

		if deps.Storage.IsNodeSyncedWithThreshold() {
			deps.Tangle.SolidifyFutureConeOfMsg(cachedMsg.GetCachedMetadata()) // meta pass +1
		}
	})
}

func attachHeartbeatEvents() {
	deps.Tangle.Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
	deps.Tangle.Events.PruningMilestoneIndexChanged.Attach(onPruningMilestoneIndexChanged)
	deps.Tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
}

func attachSolidifierGossipEvents() {
	deps.Tangle.Events.ReceivedNewMessage.Attach(onReceivedNewTx)
}

func detachHeartbeatEvents() {
	deps.Tangle.Events.SolidMilestoneIndexChanged.Detach(onSolidMilestoneIndexChanged)
	deps.Tangle.Events.PruningMilestoneIndexChanged.Detach(onPruningMilestoneIndexChanged)
	deps.Tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
}

func detachSolidifierGossipEvents() {
	deps.Tangle.Events.ReceivedNewMessage.Detach(onReceivedNewTx)
}
