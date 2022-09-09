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

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/timeutil"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
	"github.com/iotaledger/hornet/v2/pkg/profile"
	proto "github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
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
	CoreComponent = &app.CoreComponent{
		Component: &app.Component{
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
	CoreComponent *app.CoreComponent
	deps          dependencies

	// closures.
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
	SnapshotManager  *snapshot.Manager
	PruningManager   *pruning.Manager
	ServerMetrics    *metrics.ServerMetrics
	RequestQueue     gossip.RequestQueue
	MessageProcessor *gossip.MessageProcessor
	PeeringManager   *p2p.Manager
	Host             host.Host
}

func provide(c *dig.Container) error {

	if err := c.Provide(func() gossip.RequestQueue {
		return gossip.NewRequestQueue()
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	type msgProcDeps struct {
		dig.In
		Storage         *storage.Storage
		SyncManager     *syncmanager.SyncManager
		ServerMetrics   *metrics.ServerMetrics
		RequestQueue    gossip.RequestQueue
		PeeringManager  *p2p.Manager
		ProtocolManager *proto.Manager
		Profile         *profile.Profile
	}

	if err := c.Provide(func(deps msgProcDeps) *gossip.MessageProcessor {
		msgProc, err := gossip.NewMessageProcessor(
			deps.Storage,
			deps.SyncManager,
			deps.RequestQueue,
			deps.PeeringManager,
			deps.ServerMetrics,
			deps.ProtocolManager,
			&gossip.Options{
				WorkUnitCacheOpts: deps.Profile.Caches.IncomingBlocksFilter,
			})
		if err != nil {
			CoreComponent.LogPanicf("MessageProcessor initialization failed: %s", err)
		}

		return msgProc
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	type serviceDeps struct {
		dig.In
		Host            host.Host
		PeeringManager  *p2p.Manager
		Storage         *storage.Storage
		ServerMetrics   *metrics.ServerMetrics
		ProtocolManager *proto.Manager
	}

	if err := c.Provide(func(deps serviceDeps) *gossip.Service {
		return gossip.NewService(
			protocol.ID(fmt.Sprintf(iotaGossipProtocolIDTemplate, deps.ProtocolManager.Current().NetworkID())),
			deps.Host,
			deps.PeeringManager,
			deps.ServerMetrics,
			gossip.WithLogger(CoreComponent.App().NewLogger("GossipService")),
			gossip.WithUnknownPeersLimit(ParamsGossip.UnknownPeersLimit),
			gossip.WithStreamReadTimeout(ParamsGossip.StreamReadTimeout),
			gossip.WithStreamWriteTimeout(ParamsGossip.StreamWriteTimeout),
		)
	}); err != nil {
		CoreComponent.LogPanic(err)
	}

	type requesterDeps struct {
		dig.In
		Storage       *storage.Storage
		GossipService *gossip.Service
		RequestQueue  gossip.RequestQueue
	}

	if err := c.Provide(func(deps requesterDeps) *gossip.Requester {
		return gossip.NewRequester(
			deps.Storage,
			deps.GossipService,
			deps.RequestQueue,
			gossip.WithRequesterDiscardRequestsOlderThan(ParamsRequests.DiscardOlderThan),
			gossip.WithRequesterPendingRequestReEnqueueInterval(ParamsRequests.PendingReEnqueueInterval),
		)
	}); err != nil {
		CoreComponent.LogPanic(err)
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
		CoreComponent.LogPanic(err)
	}

	return nil
}

func configure() error {

	// don't re-enqueue pending requests in case the node is running hot
	deps.Requester.AddBackPressureFunc(func() bool {
		return deps.SnapshotManager.IsSnapshotting() || deps.PruningManager.IsPruning() || deps.Tangle.IsReceiveTxWorkerPoolBusy()
	})

	configureEvents()

	return nil
}

func run() error {

	if err := CoreComponent.Daemon().BackgroundWorker("GossipService", func(ctx context.Context) {
		CoreComponent.LogInfo("Running GossipService")
		attachEventsGossipService()
		deps.GossipService.Start(ctx)

		detachEventsGossipService()
		CoreComponent.LogInfo("Stopped GossipService")
	}, daemon.PriorityGossipService); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	if err := CoreComponent.Daemon().BackgroundWorker("PendingRequestsEnqueuer", func(ctx context.Context) {
		deps.Requester.RunPendingRequestEnqueuer(ctx)
	}, daemon.PriorityRequestsProcessor); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	if err := CoreComponent.Daemon().BackgroundWorker("RequestQueueDrainer", func(ctx context.Context) {
		deps.Requester.RunRequestQueueDrainer(ctx)
	}, daemon.PriorityRequestsProcessor); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	if err := CoreComponent.Daemon().BackgroundWorker("BroadcastQueue", func(ctx context.Context) {
		CoreComponent.LogInfo("Running BroadcastQueue")
		attachEventsBroadcastQueue()
		deps.Broadcaster.RunBroadcastQueueDrainer(ctx)

		detachEventsBroadcastQueue()
		CoreComponent.LogInfo("Stopped BroadcastQueue")
	}, daemon.PriorityBroadcastQueue); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	if err := CoreComponent.Daemon().BackgroundWorker("MessageProcessor", func(ctx context.Context) {
		CoreComponent.LogInfo("Running MessageProcessor")
		deps.MessageProcessor.Run(ctx)

		CoreComponent.LogInfo("Stopped MessageProcessor")
	}, daemon.PriorityMessageProcessor); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	if err := CoreComponent.Daemon().BackgroundWorker("HeartbeatBroadcaster", func(ctx context.Context) {
		ticker := timeutil.NewTicker(checkHeartbeats, checkHeartbeatsInterval, ctx)
		ticker.WaitForGracefulShutdown()
	}, daemon.PriorityHeartbeats); err != nil {
		CoreComponent.LogPanicf("failed to start worker: %s", err)
	}

	return nil
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

		case !peerRelationKnown && latestHeatbeat.SolidMilestoneIndex < snapshotInfo.PruningIndex():
			// peer is unknown or connected via autopeering and its solid milestone index is below our pruning index.
			// we can't help this neighbor to become sync, so it's better to drop the connection and free the slots for other peers.
			errUnhealthy = fmt.Errorf("peers solid milestone index is below our pruning index: %d < %d", latestHeatbeat.SolidMilestoneIndex, snapshotInfo.PruningIndex())
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
		CoreComponent.LogWarn(reason.Error())

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
		closeConnectionDueToProtocolError := func(err error) {
			CoreComponent.LogWarnf("closing connection to peer %s because of a protocol error: %s", proto.PeerID.ShortString(), err.Error())

			if err := deps.GossipService.CloseStream(proto.PeerID); err != nil {
				CoreComponent.LogWarnf("closing connection to peer %s failed, error: %s", proto.PeerID.ShortString(), err.Error())
			}
		}

		proto.Events.Errors.Hook(events.NewClosure(closeConnectionDueToProtocolError))
		proto.Parser.Events.Error.Hook(event.NewClosure(closeConnectionDueToProtocolError))

		if err := CoreComponent.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-read-%s-%s", proto.PeerID, proto.Stream.ID()), func(_ context.Context) {
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
		}, daemon.PriorityPeerGossipProtocolRead); err != nil {
			CoreComponent.LogWarnf("failed to start worker: %s", err)
		}

		if err := CoreComponent.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-write-%s-%s", proto.PeerID, proto.Stream.ID()), func(ctx context.Context) {
			// send heartbeat and latest milestone request
			if snapshotInfo := deps.Storage.SnapshotInfo(); snapshotInfo != nil {
				syncState := deps.SyncManager.SyncState()
				syncedCount := deps.GossipService.SynchronizedCount(syncState.LatestMilestoneIndex)
				connectedCount := deps.PeeringManager.ConnectedCount()
				// TODO: overflow not handled for synced/connected
				proto.SendHeartbeat(syncState.ConfirmedMilestoneIndex, snapshotInfo.PruningIndex(), syncState.LatestMilestoneIndex, byte(connectedCount), byte(syncedCount))
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
		}, daemon.PriorityPeerGossipProtocolWrite); err != nil {
			CoreComponent.LogWarnf("failed to start worker: %s", err)
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
	deps.MessageProcessor.Events.BroadcastBlock.Hook(onMessageProcessorBroadcastMessage)
}

func detachEventsGossipService() {
	deps.GossipService.Events.ProtocolStarted.Detach(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Detach(onGossipServiceProtocolTerminated)
}

func detachEventsBroadcastQueue() {
	deps.MessageProcessor.Events.BroadcastBlock.Detach(onMessageProcessorBroadcastMessage)
}
