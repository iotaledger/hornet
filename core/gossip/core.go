package gossip

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
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
)

type dependencies struct {
	dig.In
	Service          *gossip.Service
	Broadcaster      *gossip.Broadcaster
	Requester        *gossip.Requester
	Storage          *storage.Storage
	Tangle           *tangle.Tangle
	Snapshot         *snapshot.Snapshot
	ServerMetrics    *metrics.ServerMetrics
	RequestQueue     gossip.RequestQueue
	MessageProcessor *gossip.MessageProcessor
	Manager          *p2p.Manager
	Host             host.Host
}

func provide(c *dig.Container) {

	if err := c.Provide(func() gossip.RequestQueue {
		return gossip.NewRequestQueue()
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type msgProcDeps struct {
		dig.In
		Storage       *storage.Storage
		ServerMetrics *metrics.ServerMetrics
		RequestQueue  gossip.RequestQueue
		Manager       *p2p.Manager
		NodeConfig    *configuration.Configuration `name:"nodeConfig"`
		NetworkID     uint64                       `name:"networkId"`
		BelowMaxDepth int                          `name:"belowMaxDepth"`
		MinPoWScore   float64                      `name:"minPoWScore"`
		Profile       *profile.Profile
	}

	if err := c.Provide(func(deps msgProcDeps) *gossip.MessageProcessor {
		msgProc, err := gossip.NewMessageProcessor(deps.Storage, deps.RequestQueue, deps.Manager, deps.ServerMetrics, &gossip.Options{
			MinPoWScore:       deps.MinPoWScore,
			NetworkID:         deps.NetworkID,
			BelowMaxDepth:     milestone.Index(deps.BelowMaxDepth),
			WorkUnitCacheOpts: deps.Profile.Caches.IncomingMessagesFilter,
		})
		if err != nil {
			CorePlugin.Panicf("MessageProcessor initialization failed: %s", err)
		}

		return msgProc
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type serviceDeps struct {
		dig.In
		Host          host.Host
		Manager       *p2p.Manager
		Storage       *storage.Storage
		ServerMetrics *metrics.ServerMetrics
		NodeConfig    *configuration.Configuration `name:"nodeConfig"`
		NetworkID     uint64                       `name:"networkId"`
	}

	if err := c.Provide(func(deps serviceDeps) *gossip.Service {
		return gossip.NewService(
			protocol.ID(fmt.Sprintf(iotaGossipProtocolIDTemplate, deps.NetworkID)),
			deps.Host, deps.Manager, deps.ServerMetrics,
			gossip.WithLogger(logger.NewLogger("GossipService")),
			gossip.WithUnknownPeersLimit(deps.NodeConfig.Int(CfgP2PGossipUnknownPeersLimit)),
			gossip.WithStreamReadTimeout(deps.NodeConfig.Duration(CfgP2PGossipStreamReadTimeout)),
			gossip.WithStreamWriteTimeout(deps.NodeConfig.Duration(CfgP2PGossipStreamWriteTimeout)),
		)
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type requesterDeps struct {
		dig.In
		Service      *gossip.Service
		NodeConfig   *configuration.Configuration `name:"nodeConfig"`
		RequestQueue gossip.RequestQueue
		Storage      *storage.Storage
	}

	if err := c.Provide(func(deps requesterDeps) *gossip.Requester {
		return gossip.NewRequester(deps.Service,
			deps.RequestQueue,
			deps.Storage,
			gossip.WithRequesterDiscardRequestsOlderThan(deps.NodeConfig.Duration(CfgRequestsDiscardOlderThan)),
			gossip.WithRequesterPendingRequestReEnqueueInterval(deps.NodeConfig.Duration(CfgRequestsPendingReEnqueueInterval)))
	}); err != nil {
		CorePlugin.Panic(err)
	}

	type broadcasterDeps struct {
		dig.In
		Service *gossip.Service
		Manager *p2p.Manager
		Storage *storage.Storage
	}

	if err := c.Provide(func(deps broadcasterDeps) *gossip.Broadcaster {
		return gossip.NewBroadcaster(deps.Service, deps.Manager, deps.Storage, 1000)
	}); err != nil {
		CorePlugin.Panic(err)
	}
}

func configure() {

	// don't re-enqueue pending requests in case the node is running hot
	deps.Requester.AddBackPressureFunc(func() bool {
		return deps.Snapshot.IsSnapshottingOrPruning() || deps.Tangle.IsReceiveTxWorkerPoolBusy()
	})

	// register event handlers for messages
	deps.Service.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		addMessageEventHandlers(proto)

		// stream close handler
		protocolTerminated := make(chan struct{})

		deps.Service.Events.ProtocolTerminated.Attach(events.NewClosure(func(terminatedProto *gossip.Protocol) {
			if terminatedProto != proto {
				return
			}
			close(protocolTerminated)
		}))

		if err := CorePlugin.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-read-%s-%s", proto.PeerID, proto.Stream.ID()), func(_ <-chan struct{}) {
			buf := make([]byte, readBufSize)
			// only way to break out is to Reset() the stream
			for {
				r, err := proto.Read(buf)
				if err != nil {
					proto.Parser.Events.Error.Trigger(err)
					return
				}
				if _, err := proto.Parser.Read(buf[:r]); err != nil {
					return
				}
			}
		}, shutdown.PriorityPeerGossipProtocolRead); err != nil {
			CorePlugin.LogWarnf("failed to start worker: %s", err)
		}

		if err := CorePlugin.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-write-%s-%s", proto.PeerID, proto.Stream.ID()), func(shutdownSignal <-chan struct{}) {
			// send heartbeat and latest milestone request
			if snapshotInfo := deps.Storage.SnapshotInfo(); snapshotInfo != nil {
				latestMilestoneIndex := deps.Storage.LatestMilestoneIndex()
				syncedCount := deps.Service.SynchronizedCount(latestMilestoneIndex)
				connectedCount := deps.Manager.ConnectedCount(p2p.PeerRelationKnown)
				// TODO: overflow not handled for synced/connected
				proto.SendHeartbeat(deps.Storage.ConfirmedMilestoneIndex(), snapshotInfo.PruningIndex, latestMilestoneIndex, byte(connectedCount), byte(syncedCount))
				proto.SendLatestMilestoneRequest()
			}

			for {
				select {
				case <-protocolTerminated:
					return
				case <-shutdownSignal:
					return
				case data := <-proto.SendQueue:
					if err := proto.Send(data); err != nil {
						proto.Events.Errors.Trigger(err)
						return
					}
				}
			}
		}, shutdown.PriorityPeerGossipProtocolWrite); err != nil {
			CorePlugin.LogWarnf("failed to start worker: %s", err)
		}
	}))
}

func run() {

	if err := CorePlugin.Daemon().BackgroundWorker("GossipService", func(shutdownSignal <-chan struct{}) {
		CorePlugin.LogInfo("Running GossipService")
		deps.Service.Start(shutdownSignal)
		CorePlugin.LogInfo("Stopped GossipService")
	}, shutdown.PriorityGossipService); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("PendingRequestsEnqueuer", func(shutdownSignal <-chan struct{}) {
		deps.Requester.RunPendingRequestEnqueuer(shutdownSignal)
	}, shutdown.PriorityRequestsProcessor); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("RequestQueueDrainer", func(shutdownSignal <-chan struct{}) {
		deps.Requester.RunRequestQueueDrainer(shutdownSignal)
	}, shutdown.PriorityRequestsProcessor); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("BroadcastQueue", func(shutdownSignal <-chan struct{}) {
		CorePlugin.LogInfo("Running BroadcastQueue")
		onBroadcastMessage := events.NewClosure(deps.Broadcaster.Broadcast)
		deps.MessageProcessor.Events.BroadcastMessage.Attach(onBroadcastMessage)
		defer deps.MessageProcessor.Events.BroadcastMessage.Detach(onBroadcastMessage)
		deps.Broadcaster.RunBroadcastQueueDrainer(shutdownSignal)
		CorePlugin.LogInfo("Stopped BroadcastQueue")
	}, shutdown.PriorityBroadcastQueue); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("MessageProcessor", func(shutdownSignal <-chan struct{}) {
		CorePlugin.LogInfo("Running MessageProcessor")
		deps.MessageProcessor.Run(shutdownSignal)
		CorePlugin.LogInfo("Stopped MessageProcessor")
	}, shutdown.PriorityMessageProcessor); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
	}

	if err := CorePlugin.Daemon().BackgroundWorker("HeartbeatBroadcaster", func(shutdownSignal <-chan struct{}) {
		ticker := timeutil.NewTicker(checkHeartbeats, checkHeartbeatsInterval, shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityHeartbeats); err != nil {
		CorePlugin.Panicf("failed to start worker: %s", err)
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
	deps.Service.ForEach(func(proto *gossip.Protocol) bool {

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
