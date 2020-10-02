package gossip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	p2pplug "github.com/gohornet/hornet/plugins/p2p"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	// defines the send queue size of peers using the gossip protocol.
	sendQueueSize = 1500
	// defines the size of the read buffer for a gossip.Protocol stream.
	readBufSize = 2048

	heartbeatSentInterval   = 30 * time.Second
	heartbeatReceiveTimeout = 100 * time.Second

	iotaGossipProtocolIDTemplate = "/iota-gossip/%s/1.0.0"
)

var (
	PLUGIN = node.NewPlugin("Gossip", node.Enabled, configure, run)
	log    *logger.Logger

	serviceOnce sync.Once
	service     *gossip.Service

	iotaGossipProtocolID protocol.ID
)

// Service returns the gossip.Service this plugin uses.
func Service() *gossip.Service {
	serviceOnce.Do(func() {
		rQueue := gossip.NewRequestQueue()
		msgProc := gossip.NewMessageProcessor(rQueue, p2pplug.PeeringService(), &gossip.Options{
			ValidMWM:          config.NodeConfig.GetUint64(config.CfgCoordinatorMWM),
			WorkUnitCacheOpts: profile.LoadProfile().Caches.IncomingMessagesFilter,
		})
		// TODO: fix relation between gossip.Service and gossip.BroadcastQueue
		service = gossip.NewService(msgProc, p2pplug.PeeringService(), rQueue)
	})
	return service
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// instantiate gossip service
	gossipService := Service()

	// setup event handlers to instantiate/destroy gossip protocol streams
	cooPubKey := config.NodeConfig.GetString(config.CfgCoordinatorPublicKey)
	iotaGossipProtocolID = protocol.ID(fmt.Sprintf(iotaGossipProtocolIDTemplate, cooPubKey[:5]))

	// handles inbound streams from remote peer
	p2pplug.Host().SetStreamHandler(iotaGossipProtocolID, registerGossipProtocolStream)
	peeringService := p2pplug.PeeringService()

	// handles outbound stream creation
	peeringService.Events.Connected.Attach(events.NewClosure(initGossipStreamIfOutbound))

	// handles stream unregistering
	peeringService.Events.Disconnected.Attach(events.NewClosure(unregisterGossipProtocolStream))

	// register event handlers for messages
	gossipService.Events.Created.Attach(events.NewClosure(func(proto *gossip.Protocol) {
		log.Infof("starting new gossip protocol stream with %s", proto.PeerID)

		addMessageEventHandlers(proto)

		// send heartbeat and latest milestone request
		if snapshotInfo := tangle.GetSnapshotInfo(); snapshotInfo != nil {
			latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
			syncedCount := gossipService.SynchronizedCount(latestMilestoneIndex)
			connectedCount := peeringService.ConnectedPeerCount()
			// TODO: overflow not handled for synced/connected
			proto.SendHeartbeat(tangle.GetSolidMilestoneIndex(), snapshotInfo.PruningIndex, latestMilestoneIndex, byte(connectedCount), byte(syncedCount))
			proto.SendLatestMilestoneRequest()
		}

		// reset the gossip protocol stream when the peer disconnects
		disconnected := make(chan struct{})
		p2pplug.PeeringService().Peer(proto.PeerID).Events.Disconnected.Attach(events.NewClosure(func() {
			removeMessageEventHandlers(proto)
			close(disconnected)
			// a stream needs to be reset instead of Close() otherwise
			// the Read() hangs forever.
			_ = proto.Stream.Reset()
		}))

		daemon.BackgroundWorker(fmt.Sprintf("gossip-protocol-read-%s", proto.PeerID), func(shutdownSignal <-chan struct{}) {
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

		daemon.BackgroundWorker(fmt.Sprintf("gossip-protocol-write-%s", proto.PeerID), func(shutdownSignal <-chan struct{}) {
			for {
				select {
				case <-disconnected:
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

func run(_ *node.Plugin) {

	daemon.BackgroundWorker("BroadcastQueue", func(shutdownSignal <-chan struct{}) {
		log.Info("Running BroadcastQueue")
		Service().RunBroadcast(shutdownSignal)
		log.Info("Stopped BroadcastQueue")
	}, shutdown.PriorityBroadcastQueue)

	daemon.BackgroundWorker("MessageProcessor", func(shutdownSignal <-chan struct{}) {
		log.Info("Running MessageProcessor")
		onBroadcastMessage := events.NewClosure(Service().Broadcast)
		msgProc := Service().MessageProcessor
		msgProc.Events.BroadcastMessage.Attach(onBroadcastMessage)
		msgProc.Run(shutdownSignal)
		msgProc.Events.BroadcastMessage.Detach(onBroadcastMessage)
		log.Info("Stopped MessageProcessor")
	}, shutdown.PriorityMessageProcessor)

	daemon.BackgroundWorker("HeartbeatBroadcaster", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(checkHeartbeats, 5*time.Second, shutdownSignal)
	}, shutdown.PriorityHeartbeats)

	runRequestWorkers()
}

// initializes a gossip stream if the connection to the given peer is outbound.
func initGossipStreamIfOutbound(p *p2p.Peer) {
	conns := p2pplug.Host().Network().ConnsToPeer(p.ID)
	if conns[0].Stat().Direction != network.DirOutbound {
		return
	}

	log.Infof("starting protocol %s with %s", iotaGossipProtocolID, p.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := p2pplug.Host().NewStream(ctx, p.ID, iotaGossipProtocolID)
	if err != nil {
		log.Errorf("unable to start %s protocol with %s", iotaGossipProtocolID, p.ID)
		// TODO: close connection?
		return
	}

	registerGossipProtocolStream(stream)
}

// unregisters the gossip stream for the given peer
func unregisterGossipProtocolStream(p *p2p.Peer) {
	Service().Unregister(p.ID)
}

// inits the gossip.Protocol and registers it with the gossip.Service.
func registerGossipProtocolStream(stream network.Stream) {
	remotePeer := p2pplug.PeeringService().Peer(stream.Conn().RemotePeer())
	proto := gossip.NewProtocol(remotePeer.ID, stream, sendQueueSize)
	if !Service().Register(proto) {
		log.Warn("%s wanted to initiate a gossip protocol stream while it already had one", remotePeer.ID)
		return
	}
	Service().Events.Created.Trigger(proto)
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
	peersToReconnect := make(map[peer.ID]*p2p.Peer)

	// check if peers are alive by checking whether we received heartbeats lately
	p2pplug.PeeringService().ForAllConnected(func(p *p2p.Peer) bool {
		proto := Service().Protocol(p.ID)
		if proto == nil {
			return true
		}

		if time.Since(proto.HeartbeatReceivedTime) < heartbeatReceiveTimeout {
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
		log.Infof("closing connection to neighbor %s because we didn't receive heartbeats anymore", p.ID)
		peersToReconnect[p.ID] = p
		return true
	})

	/*
		// TODO: re-introduce once p2p discovery is implemented
			for peerIDToRemove := range peerIDsToRemove {
				peering.Manager().Remove(peerIDToRemove)
			}
	*/

	for _, p := range peersToReconnect {
		p.Disconnect()
	}

}
