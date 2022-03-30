package gossip

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// defines the size of the read buffer for a gossip.Protocol stream.
	readBufSize = 2048

	heartbeatSentInterval   = 30 * time.Second
	heartbeatReceiveTimeout = 100 * time.Second
	checkHeartbeatsInterval = 5 * time.Second

	iotaGossipProtocolIDTemplate = "/iota-gossip/%d/1.0.0"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name:      "Gossip",
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

	// closures
	onGossipServiceProtocolStarted     *events.Closure
	onGossipServiceProtocolTerminated  *events.Closure
	onMessageProcessorBroadcastMessage *events.Closure
)

type dependencies struct {
	dig.In
	GossipService    *gossip.Service
	Broadcaster      *gossip.Broadcaster
	Requester        *gossip.Requester
	Storage          *storage.Storage
	SyncManager      *syncmanager.SyncManager
	Tangle           *tangle.Tangle
	SnapshotManager  *snapshot.SnapshotManager
	ServerMetrics    *metrics.ServerMetrics
	RequestQueue     gossip.RequestQueue
	MessageProcessor *gossip.MessageProcessor
	PeeringManager   *p2p.Manager
	Host             host.Host
}

func provide(c *dig.Container) {

	if err := c.Provide(func() gossip.RequestQueue {
		return gossip.NewRequestQueue()
	}); err != nil {
		CorePlugin.LogPanic(err)
	}

	type msgProcDeps struct {
		dig.In
		Storage                   *storage.Storage
		SyncManager               *syncmanager.SyncManager
		ServerMetrics             *metrics.ServerMetrics
		RequestQueue              gossip.RequestQueue
		PeeringManager            *p2p.Manager
		NodeConfig                *configuration.Configuration `name:"nodeConfig"`
		NetworkID                 uint64                       `name:"networkId"`
		DeserializationParameters *iotago.DeSerializationParameters
		BelowMaxDepth             int     `name:"belowMaxDepth"`
		MinPoWScore               float64 `name:"minPoWScore"`
		Profile                   *profile.Profile
	}

	if err := c.Provide(func(deps msgProcDeps) *gossip.MessageProcessor {
		msgProc, err := gossip.NewMessageProcessor(
			deps.Storage,
			deps.SyncManager,
			deps.RequestQueue,
			deps.PeeringManager,
			deps.ServerMetrics,
			deps.DeserializationParameters,
			&gossip.Options{
				MinPoWScore:       deps.MinPoWScore,
				ProtocolVersion:   iotago.ProtocolVersion,
				NetworkID:         deps.NetworkID,
				BelowMaxDepth:     milestone.Index(deps.BelowMaxDepth),
				WorkUnitCacheOpts: deps.Profile.Caches.IncomingMessagesFilter,
			})
		if err != nil {
			CorePlugin.LogPanicf("MessageProcessor initialization failed: %s", err)
		}

		return msgProc
	}); err != nil {
		CorePlugin.LogPanic(err)
	}

	type serviceDeps struct {
		dig.In
		Host           host.Host
		PeeringManager *p2p.Manager
		Storage        *storage.Storage
		ServerMetrics  *metrics.ServerMetrics
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
		NetworkID      uint64                       `name:"networkId"`
	}

	if err := c.Provide(func(deps serviceDeps) *gossip.Service {
		return gossip.NewService(
			protocol.ID(fmt.Sprintf(iotaGossipProtocolIDTemplate, deps.NetworkID)),
			deps.Host,
			deps.PeeringManager,
			deps.ServerMetrics,
			gossip.WithLogger(logger.NewLogger("GossipService")),
			gossip.WithUnknownPeersLimit(deps.NodeConfig.Int(CfgP2PGossipUnknownPeersLimit)),
			gossip.WithStreamReadTimeout(deps.NodeConfig.Duration(CfgP2PGossipStreamReadTimeout)),
			gossip.WithStreamWriteTimeout(deps.NodeConfig.Duration(CfgP2PGossipStreamWriteTimeout)),
		)
	}); err != nil {
		CorePlugin.LogPanic(err)
	}

	type requesterDeps struct {
		dig.In
		NodeConfig    *configuration.Configuration `name:"nodeConfig"`
		Storage       *storage.Storage
		GossipService *gossip.Service
		RequestQueue  gossip.RequestQueue
	}

	if err := c.Provide(func(deps requesterDeps) *gossip.Requester {
		return gossip.NewRequester(
			deps.Storage,
			deps.GossipService,
			deps.RequestQueue,
			gossip.WithRequesterDiscardRequestsOlderThan(deps.NodeConfig.Duration(CfgRequestsDiscardOlderThan)),
			gossip.WithRequesterPendingRequestReEnqueueInterval(deps.NodeConfig.Duration(CfgRequestsPendingReEnqueueInterval)))
	}); err != nil {
		CorePlugin.LogPanic(err)
	}

	type broadcasterDeps struct {
		dig.In
		Storage        *storage.Storage
		SyncManager    *syncmanager.SyncManager
		PeeringManager *p2p.Manager
		GossipService  *gossip.Service
	}

	if err := c.Provide(func(deps broadcasterDeps) *gossip.Broadcaster {
		return gossip.NewBroadcaster(
			deps.Storage,
			deps.SyncManager,
			deps.PeeringManager,
			deps.GossipService,
			1000)
	}); err != nil {
		CorePlugin.LogPanic(err)
	}
}

func configure() {

	// don't re-enqueue pending requests in case the node is running hot
	deps.Requester.AddBackPressureFunc(func() bool {
		return deps.SnapshotManager.IsSnapshottingOrPruning() || deps.Tangle.IsReceiveTxWorkerPoolBusy()
	})

	configureEvents()
}

func run() {

	if err := CorePlugin.Daemon().BackgroundWorker("GossipService", func(ctx context.Context) {
		CorePlugin.LogInfo("Running GossipService")
		attachEventsGossipService()
		deps.GossipService.Start(ctx)

		detachEventsGossipService()
		CorePlugin.LogInfo("Stopped GossipService")
	}, shutdown.PriorityGossipService); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("PendingRequestsEnqueuer", func(ctx context.Context) {
		deps.Requester.RunPendingRequestEnqueuer(ctx)
	}, shutdown.PriorityRequestsProcessor); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("RequestQueueDrainer", func(ctx context.Context) {
		deps.Requester.RunRequestQueueDrainer(ctx)
	}, shutdown.PriorityRequestsProcessor); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("BroadcastQueue", func(ctx context.Context) {
		CorePlugin.LogInfo("Running BroadcastQueue")
		attachEventsBroadcastQueue()
		deps.Broadcaster.RunBroadcastQueueDrainer(ctx)

		detachEventsBroadcastQueue()
		CorePlugin.LogInfo("Stopped BroadcastQueue")
	}, shutdown.PriorityBroadcastQueue); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("MessageProcessor", func(ctx context.Context) {
		CorePlugin.LogInfo("Running MessageProcessor")
		deps.MessageProcessor.Run(ctx)

		CorePlugin.LogInfo("Stopped MessageProcessor")
	}, shutdown.PriorityMessageProcessor); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("HeartbeatBroadcaster", func(ctx context.Context) {
		ticker := timeutil.NewTicker(checkHeartbeats, checkHeartbeatsInterval, ctx)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityHeartbeats); err != nil {
		CorePlugin.LogPanicf("failed to start worker: %s", err)
	}
}

// checkHeartbeats sends a heartbeat to each peer and also checks
// whether we received heartbeats from other peers. if a peer didn't send any
// heartbeat for a defined period of time, then the connection to it is dropped.
func checkHeartbeats() {

	// send a new heartbeat message to every neighbor at least every heartbeatSentInterval
	deps.Broadcaster.BroadcastHeartbeat(func(proto *gossip.Protocol) bool {
		return time.Since(proto.HeartbeatSentTime) > heartbeatSentInterval
	})

	peersToReconnect := make(map[peer.ID]struct{})

	// check if peers are alive by checking whether we received heartbeats lately
	deps.GossipService.ForEach(func(proto *gossip.Protocol) bool {

		// use a grace period before the heartbeat check is applied
		if time.Since(proto.Stream.Stat().Opened) <= checkHeartbeatsInterval ||
			time.Since(proto.HeartbeatReceivedTime) < heartbeatReceiveTimeout {
			return true
		}

		/*
			// TODO: re-introduce once p2p discovery is implemented
			// peer is connected but doesn't seem to be alive
			if p.Autopeering != nil {
				// it's better to drop the connection to autopeered peers and free the slots for other peers
				peerIDsToRemove[p.ID] = struct{}{}
				CorePlugin.LogInfof("dropping autopeered neighbor %s / %s because we didn't receive heartbeats anymore", p.Autopeering.ID(), p.Autopeering.ID())
				return true
			}
		*/

		// close the connection to static connected peers, so they will be moved into reconnect pool to reestablish the connection
		CorePlugin.LogInfof("closing connection to peer %s because we didn't receive heartbeats anymore", proto.PeerID.ShortString())
		peersToReconnect[proto.PeerID] = struct{}{}
		return true
	})

	/*
		// TODO: re-introduce once p2p discovery is implemented
		for peerIDToRemove := range peerIDsToRemove {
			peering.Manager().Remove(peerIDToRemove)
		}
	*/

	// close the connection to the peers to trigger a reconnect
	for p := range peersToReconnect {
		conns := deps.Host.Network().ConnsToPeer(p)
		for _, conn := range conns {
			_ = conn.Close()
		}
	}
}

func configureEvents() {

	onGossipServiceProtocolStarted = events.NewClosure(func(proto *gossip.Protocol) {
		attachEventsProtocolMessages(proto)

		// attach protocol errors
		closeConnectionDueToProtocolError := events.NewClosure(func(err error) {
			CorePlugin.LogWarnf("closing connection to peer %s because of a protocol error: %s", proto.PeerID.ShortString(), err.Error())

			if err := deps.GossipService.CloseStream(proto.PeerID); err != nil {
				CorePlugin.LogWarnf("closing connection to peer %s failed, error: %s", proto.PeerID.ShortString(), err.Error())
			}
		})

		proto.Events.Errors.Attach(closeConnectionDueToProtocolError)
		proto.Parser.Events.Error.Attach(closeConnectionDueToProtocolError)

		if err := CorePlugin.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-read-%s-%s", proto.PeerID, proto.Stream.ID()), func(_ context.Context) {
			buf := make([]byte, readBufSize)
			// only way to break out is to Reset() the stream
			for {
				r, err := proto.Read(buf)
				if err != nil {
					// proto.Events.Error is already triggered inside Read
					return
				}
				if _, err := proto.Parser.Read(buf[:r]); err != nil {
					// proto.Events.Error is already triggered inside Read
					return
				}
			}
		}, shutdown.PriorityPeerGossipProtocolRead); err != nil {
			CorePlugin.LogWarnf("failed to start worker: %s", err)
		}

		if err := CorePlugin.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-write-%s-%s", proto.PeerID, proto.Stream.ID()), func(ctx context.Context) {
			// send heartbeat and latest milestone request
			if snapshotInfo := deps.Storage.SnapshotInfo(); snapshotInfo != nil {
				latestMilestoneIndex := deps.SyncManager.LatestMilestoneIndex()
				syncedCount := deps.GossipService.SynchronizedCount(latestMilestoneIndex)
				connectedCount := deps.PeeringManager.ConnectedCount()
				// TODO: overflow not handled for synced/connected
				proto.SendHeartbeat(deps.SyncManager.ConfirmedMilestoneIndex(), snapshotInfo.PruningIndex, latestMilestoneIndex, byte(connectedCount), byte(syncedCount))
				proto.SendLatestMilestoneRequest()
			}

			for {
				select {
				case <-proto.Terminated():
					return
				case <-ctx.Done():
					return
				case data := <-proto.SendQueue:
					if err := proto.Send(data); err != nil {
						return
					}
				}
			}
		}, shutdown.PriorityPeerGossipProtocolWrite); err != nil {
			CorePlugin.LogWarnf("failed to start worker: %s", err)
		}
	})

	onGossipServiceProtocolTerminated = events.NewClosure(func(proto *gossip.Protocol) {
		if proto == nil {
			return
		}

		detachEventsProtocolMessages(proto)

		// detach protocol errors
		if proto.Events != nil && proto.Events.Errors != nil {
			proto.Events.Errors.DetachAll()
		}

		if proto.Parser != nil && proto.Parser.Events.Error != nil {
			proto.Parser.Events.Error.DetachAll()
		}
	})

	onMessageProcessorBroadcastMessage = events.NewClosure(deps.Broadcaster.Broadcast)
}

func attachEventsGossipService() {
	deps.GossipService.Events.ProtocolStarted.Attach(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Attach(onGossipServiceProtocolTerminated)
}

func attachEventsBroadcastQueue() {
	deps.MessageProcessor.Events.BroadcastMessage.Attach(onMessageProcessorBroadcastMessage)
}

func detachEventsGossipService() {
	deps.GossipService.Events.ProtocolStarted.Detach(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Detach(onGossipServiceProtocolTerminated)
}

func detachEventsBroadcastQueue() {
	deps.MessageProcessor.Events.BroadcastMessage.Detach(onMessageProcessorBroadcastMessage)
}
