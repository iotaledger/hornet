package gossip

import (
	"fmt"
	"sync"
	"time"

	"github.com/gohornet/hornet/core/database"
	p2pcore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	// defines the size of the read buffer for a gossip.Protocol stream.
	readBufSize = 2048

	heartbeatSentInterval   = 30 * time.Second
	heartbeatReceiveTimeout = 100 * time.Second
	checkHeartbeatsInterval = 5 * time.Second

	iotaGossipProtocolIDTemplate = "/iota-gossip/%d/1.0.0"
)

var (
	CoreModule *node.CoreModule
	log        *logger.Logger

	serviceOnce sync.Once
	service     *gossip.Service

	msgProcOnce sync.Once
	msgProc     *gossip.MessageProcessor

	rQueueOnce sync.Once
	rQueue     gossip.RequestQueue
)

// Service returns the gossip.Service instance.
func Service() *gossip.Service {
	serviceOnce.Do(func() {
		// ToDo: Issa scam! Snapshot info is not yet known here (because snapshot is loaded afterwards)
		//networkID := tangle.GetSnapshotInfo().NetworkID)
		var networkID uint8 = 1
		iotaGossipProtocolID := protocol.ID(fmt.Sprintf(iotaGossipProtocolIDTemplate, networkID))
		service = gossip.NewService(iotaGossipProtocolID, p2pcore.Host(), p2pcore.Manager(),
			gossip.WithLogger(logger.NewLogger("GossipService")),
			gossip.WithUnknownPeersLimit(config.NodeConfig.Int(config.CfgP2PGossipUnknownPeersLimit)),
		)
	})
	return service
}

// MessageProcessor returns the gossip.MessageProcessor instance.
func MessageProcessor() *gossip.MessageProcessor {
	msgProcOnce.Do(func() {
		RequestQueue()
		msgProc = gossip.NewMessageProcessor(database.Tangle(), rQueue, p2pcore.Manager(), &gossip.Options{
			ValidMWM:          uint64(config.NodeConfig.Int64(config.CfgCoordinatorMWM)),
			WorkUnitCacheOpts: profile.LoadProfile().Caches.IncomingMessagesFilter,
		})
	})
	return msgProc
}

// RequestQueue returns the gossip.RequestQueue instance.
func RequestQueue() gossip.RequestQueue {
	rQueueOnce.Do(func() {
		rQueue = gossip.NewRequestQueue()
	})
	return rQueue
}

func init() {
	CoreModule = node.NewCoreModule("Gossip", configure, run)
}

func configure(coreModule *node.CoreModule) {
	log = logger.NewLogger(coreModule.Name)
	gossipService := Service()
	MessageProcessor()

	// register event handlers for messages
	gossipService.Events.ProtocolStarted.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		addMessageEventHandlers(proto)

		// stream close handler
		protocolTerminated := make(chan struct{})

		gossipService.Events.ProtocolTerminated.Attach(events.NewClosure(func(terminatedProto *gossip.Protocol) {
			if terminatedProto != proto {
				return
			}
			close(protocolTerminated)
		}))

		_ = CoreModule.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-read-%s-%s", proto.PeerID, proto.Stream.ID()), func(shutdownSignal <-chan struct{}) {
			buf := make([]byte, readBufSize)
			// only way to break out is to Reset() the stream
			for {
				r, err := proto.Stream.Read(buf)
				if err != nil {
					proto.Parser.Events.Error.Trigger(err)
					return
				}
				if _, err := proto.Parser.Read(buf[:r]); err != nil {
					return
				}
			}
		}, shutdown.PriorityPeerGossipProtocolRead)

		_ = CoreModule.Daemon().BackgroundWorker(fmt.Sprintf("gossip-protocol-write-%s-%s", proto.PeerID, proto.Stream.ID()), func(shutdownSignal <-chan struct{}) {
			// send heartbeat and latest milestone request
			if snapshotInfo := database.Tangle().GetSnapshotInfo(); snapshotInfo != nil {
				latestMilestoneIndex := database.Tangle().GetLatestMilestoneIndex()
				syncedCount := gossipService.SynchronizedCount(latestMilestoneIndex)
				connectedCount := p2pcore.Manager().ConnectedCount(p2p.PeerRelationKnown)
				// TODO: overflow not handled for synced/connected
				proto.SendHeartbeat(database.Tangle().GetSolidMilestoneIndex(), snapshotInfo.PruningIndex, latestMilestoneIndex, byte(connectedCount), byte(syncedCount))
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
					}
				}
			}
		}, shutdown.PriorityPeerGossipProtocolWrite)
	}))
}

func run(_ *node.CoreModule) {

	_ = CoreModule.Daemon().BackgroundWorker("GossipService", func(shutdownSignal <-chan struct{}) {
		log.Info("Running GossipService")
		Service().Start(shutdownSignal)
		log.Info("Stopped GossipService")
	}, shutdown.PriorityGossipService)

	_ = CoreModule.Daemon().BackgroundWorker("BroadcastQueue", func(shutdownSignal <-chan struct{}) {
		log.Info("Running BroadcastQueue")
		broadcastQueue := make(chan *gossip.Broadcast)
		onBroadcastMessage := events.NewClosure(func(b *gossip.Broadcast) {
			broadcastQueue <- b
		})
		msgProc.Events.BroadcastMessage.Attach(onBroadcastMessage)
		defer msgProc.Events.BroadcastMessage.Detach(onBroadcastMessage)
	exit:
		for {
			select {
			case <-shutdownSignal:
				break exit
			case b := <-broadcastQueue:
				Service().ForEach(func(proto *gossip.Protocol) bool {
					if _, excluded := b.ExcludePeers[proto.PeerID]; excluded {
						return true
					}

					proto.SendMessage(b.MsgData)
					return true
				})
			}
		}
		log.Info("Stopped BroadcastQueue")
	}, shutdown.PriorityBroadcastQueue)

	_ = CoreModule.Daemon().BackgroundWorker("MessageProcessor", func(shutdownSignal <-chan struct{}) {
		log.Info("Running MessageProcessor")
		MessageProcessor().Run(shutdownSignal)
		log.Info("Stopped MessageProcessor")
	}, shutdown.PriorityMessageProcessor)

	_ = CoreModule.Daemon().BackgroundWorker("HeartbeatBroadcaster", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(checkHeartbeats, checkHeartbeatsInterval, shutdownSignal)
	}, shutdown.PriorityHeartbeats)

	runRequestWorkers()
}

// checkHeartbeats sends a heartbeat to each peer and also checks
// whether we received heartbeats from other peers. if a peer didn't send any
// heartbeat for a defined period of time, then the connection to it is dropped.
func checkHeartbeats() {

	// send a new heartbeat message to every neighbor at least every heartbeatSentInterval
	BroadcastHeartbeat(func(proto *gossip.Protocol) bool {
		return time.Since(proto.HeartbeatSentTime) > heartbeatSentInterval
	})

	//peerIDsToRemove := make(map[string]struct{})
	peersToReconnect := make(map[peer.ID]struct{})

	// check if peers are alive by checking whether we received heartbeats lately
	Service().ForEach(func(proto *gossip.Protocol) bool {

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
				log.Infof("dropping autopeered neighbor %s / %s because we didn't receive heartbeats anymore", p.Autopeering.ID(), p.Autopeering.ID())
				return true
			}
		*/

		// close the connection to static connected peers, so they will be moved into reconnect pool to reestablish the connection
		log.Infof("closing connection to peer %s because we didn't receive heartbeats anymore", proto.PeerID.ShortString())
		peersToReconnect[proto.PeerID] = struct{}{}
		return true
	})

	/*
		// TODO: re-introduce once p2p discovery is implemented
		for peerIDToRemove := range peerIDsToRemove {
			peering.Manager().Remove(peerIDToRemove)
		}
	*/

	for p := range peersToReconnect {
		conns := p2pcore.Host().Network().ConnsToPeer(p)
		for _, conn := range conns {
			_ = conn.Close()
		}
	}
}
