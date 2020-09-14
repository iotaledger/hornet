package tangle

import (
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/iota.go/address"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/database"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/peering"
)

const (
	HeartbeatSentInterval   = 30 * time.Second
	HeartbeatReceiveTimeout = 100 * time.Second
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

	if err := address.ValidAddress(config.NodeConfig.GetString(config.CfgCoordinatorAddress)); err != nil {
		log.Fatal(err.Error())
	}

	tangle.ConfigureMilestones(
		hornet.HashFromAddressTrytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress)),
		config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel),
		uint64(config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth)),
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

	daemon.BackgroundWorker("Tangle[Heartbeats]", func(shutdownSignal <-chan struct{}) {
		attachHeartbeatEvents()

		checkHeartbeats := func() {
			// send a new heartbeat message to every neighbor at least every HeartbeatSentInterval
			gossip.BroadcastHeartbeat(func(p *peer.Peer) bool {
				return time.Since(p.HeartbeatSentTime) > HeartbeatSentInterval
			})

			peerIDsToRemove := make(map[string]struct{})
			peersToReconnect := make(map[string]*peer.Peer)

			// check if peers are alive by checking whether we received heartbeats lately
			peering.Manager().ForAllConnected(func(p *peer.Peer) bool {
				if !p.Protocol.Supports(sting.FeatureSet) {
					return true
				}

				if time.Since(p.HeartbeatReceivedTime) < HeartbeatReceiveTimeout {
					return true
				}

				// peer is connected but doesn't seem to be alive
				if p.Autopeering != nil {
					// it's better to drop the connection to autopeered peers and free the slots for other peers
					peerIDsToRemove[p.ID] = struct{}{}
					log.Infof("dropping autopeered neighbor %s / %s because we didn't receive heartbeats anymore", p.Autopeering.Address(), p.Autopeering.ID())
					return true
				}

				// close the connection to static connected peers, so they will be moved into reconnect pool to reestablish the connection
				log.Infof("closing connection to neighbor %s because we didn't receive heartbeats anymore", p.ID)
				peersToReconnect[p.ID] = p
				return true
			})

			for peerIDToRemove := range peerIDsToRemove {
				peering.Manager().Remove(peerIDToRemove)
			}

			for _, p := range peersToReconnect {
				p.Conn.Close()
			}

		}
		timeutil.Ticker(checkHeartbeats, 5*time.Second, shutdownSignal)

		detachHeartbeatEvents()
	}, shutdown.PriorityHeartbeats)

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
		timeutil.Ticker(printStatus, 1*time.Second, shutdownSignal)
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

	onReceivedNewTx = events.NewClosure(func(cachedTx *tangle.CachedTransaction, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		// Force release possible here, since processIncomingTx still holds a reference
		defer cachedTx.Release(true) // tx -1

		if tangle.IsNodeSyncedWithThreshold() {
			solidifyFutureConeOfTx(cachedTx.GetCachedMetadata()) // meta pass +1
		}
	})
}

func attachHeartbeatEvents() {
	Events.SolidMilestoneIndexChanged.Attach(onSolidMilestoneIndexChanged)
	Events.PruningMilestoneIndexChanged.Attach(onPruningMilestoneIndexChanged)
	Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
}

func attachSolidifierGossipEvents() {
	Events.ReceivedNewTransaction.Attach(onReceivedNewTx)
}

func detachHeartbeatEvents() {
	Events.SolidMilestoneChanged.Detach(onSolidMilestoneIndexChanged)
	Events.PruningMilestoneIndexChanged.Detach(onPruningMilestoneIndexChanged)
	Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)
}

func detachSolidifierGossipEvents() {
	Events.ReceivedNewTransaction.Detach(onReceivedNewTx)
}

// SetUpdateSyncedAtStartup sets the flag if the isNodeSynced status should be updated at startup
func SetUpdateSyncedAtStartup(updateSynced bool) {
	updateSyncedAtStartup = updateSynced
}
