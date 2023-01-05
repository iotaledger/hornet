package gossip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/node"
	"github.com/iotaledger/hornet/pkg/p2p"
	"github.com/iotaledger/hornet/pkg/profile"
	"github.com/iotaledger/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/pkg/shutdown"
	"github.com/iotaledger/hornet/pkg/snapshot"
	"github.com/iotaledger/hornet/pkg/tangle"
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
		Storage        *storage.Storage
		SyncManager    *syncmanager.SyncManager
		ServerMetrics  *metrics.ServerMetrics
		RequestQueue   gossip.RequestQueue
		PeeringManager *p2p.Manager
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
		NetworkID      uint64                       `name:"networkId"`
		BelowMaxDepth  int                          `name:"belowMaxDepth"`
		MinPoWScore    float64                      `name:"minPoWScore"`
		Profile        *profile.Profile
	}

	if err := c.Provide(func(deps msgProcDeps) *gossip.MessageProcessor {
		msgProc, err := gossip.NewMessageProcessor(
			deps.Storage,
			deps.SyncManager,
			deps.RequestQueue,
			deps.PeeringManager,
			deps.ServerMetrics,
			&gossip.Options{
				MinPoWScore:       deps.MinPoWScore,
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

	peersToRemove := make(map[peer.ID]error)
	peersToReconnect := make(map[peer.ID]error)

	snapshotInfo := deps.Storage.SnapshotInfo()
	if snapshotInfo == nil {
		// we can't check the health of the peers without info about pruning index
		return
	}

	// check if peers are alive by checking whether we received heartbeats lately
	deps.GossipService.ForEach(func(proto *gossip.Protocol) bool {

		protoStream := proto.Stream
		if protoStream == nil {
			// stream not established yet
			return true
		}

		latestHeatbeat := proto.LatestHeartbeat
		if latestHeatbeat == nil && time.Since(protoStream.Stat().Opened) <= checkHeartbeatsInterval {
			// use a grace period before the heartbeat check is applied
			return true
		}

		var peerRelationKnown bool
		deps.PeeringManager.Call(proto.PeerID, func(peer *p2p.Peer) {
			peerRelationKnown = peer.Relation == p2p.PeerRelationKnown
		})

		var errUnhealthy error
		switch {
		case latestHeatbeat == nil:
			// no heartbeat received in the grace period
			errUnhealthy = errors.New("no heartbeat received in grace period")

		case time.Since(proto.HeartbeatReceivedTime) > heartbeatReceiveTimeout:
			// heartbeat outdated
			errUnhealthy = errors.New("heartbeat outdated")

		case !peerRelationKnown && latestHeatbeat.SolidMilestoneIndex < snapshotInfo.PruningIndex:
			// peer is unknown or connected via autopeering and its solid milestone index is below our pruning index.
			// we can't help this neighbor to become sync, so it's better to drop the connection and free the slots for other peers.
			errUnhealthy = fmt.Errorf("peers solid milestone index is below our pruning index: %d < %d", latestHeatbeat.SolidMilestoneIndex, snapshotInfo.PruningIndex)
		}

		if errUnhealthy == nil {
			// peer is healthy
			return true
		}

		if peerRelationKnown {
			// close the connection to static connected peers, so they will be moved into reconnect pool to reestablish the connection
			peersToReconnect[proto.PeerID] = fmt.Errorf("dropping connection to peer %s, error: %w", proto.PeerID.ShortString(), errUnhealthy)
		} else {
			// it's better to drop the connection to unknown and autopeered peers and free the slots for other peers
			peersToRemove[proto.PeerID] = fmt.Errorf("dropping connection to peer %s, error: %w", proto.PeerID.ShortString(), errUnhealthy)
		}

		return true
	})

	// drop the connection to the peers
	for p, reason := range peersToRemove {
		_ = deps.PeeringManager.DisconnectPeer(p, reason)
	}

	// close the connection to the peers to trigger a reconnect
	for p, reason := range peersToReconnect {
		CorePlugin.LogWarn(reason.Error())

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

		proto.Events.Errors.Hook(closeConnectionDueToProtocolError)
		proto.Parser.Events.Error.Hook(closeConnectionDueToProtocolError)

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
				syncState := deps.SyncManager.SyncState()
				syncedCount := deps.GossipService.SynchronizedCount(syncState.LatestMilestoneIndex)
				connectedCount := deps.PeeringManager.ConnectedCount()
				// TODO: overflow not handled for synced/connected
				proto.SendHeartbeat(syncState.ConfirmedMilestoneIndex, snapshotInfo.PruningIndex, syncState.LatestMilestoneIndex, byte(connectedCount), byte(syncedCount))
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
	deps.GossipService.Events.ProtocolStarted.Hook(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Hook(onGossipServiceProtocolTerminated)
}

func attachEventsBroadcastQueue() {
	deps.MessageProcessor.Events.BroadcastMessage.Hook(onMessageProcessorBroadcastMessage)
}

func detachEventsGossipService() {
	deps.GossipService.Events.ProtocolStarted.Detach(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Detach(onGossipServiceProtocolTerminated)
}

func detachEventsBroadcastQueue() {
	deps.MessageProcessor.Events.BroadcastMessage.Detach(onMessageProcessorBroadcastMessage)
}
