package peering

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/peering"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol"
	"github.com/gohornet/hornet/pkg/protocol/handshake"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	ExamplePeerURI = "example.neighbor.com:15600"
)

var (
	PLUGIN      = node.NewPlugin("Peering", node.Enabled, configure, run)
	log         *logger.Logger
	manager     *peering.Manager
	managerOnce sync.Once
)

// Manager gets the peering Manager instance the peering plugin uses.
func Manager() *peering.Manager {
	managerOnce.Do(func() {
		// init protocol package with handshake data
		cooAddrBytes := hornet.HashFromAddressTrytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress))
		mwm := config.NodeConfig.GetInt(config.CfgCoordinatorMWM)
		bindAddr := config.NodeConfig.GetString(config.CfgNetGossipBindAddress)
		if err := protocol.Init(cooAddrBytes, mwm, bindAddr); err != nil {
			log.Fatalf("couldn't initialize protocol: %s", err)
		}

		// load initial config peers
		var peers []*config.PeerConfig
		if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &peers); err != nil {
			panic(err)
		}

		for i, p := range peers {
			if p.ID == ExamplePeerURI {
				peers[i] = peers[len(peers)-1]
				peers[len(peers)-1] = nil
				peers = peers[:len(peers)-1]
				break
			}
		}

		// init peer manager
		manager = peering.NewManager(peering.Options{
			BindAddress: config.NodeConfig.GetString(config.CfgNetGossipBindAddress),
			ValidHandshake: handshake.Handshake{
				ByteEncodedCooAddress: cooAddrBytes,
				MWM:                   byte(mwm),
			},
			MaxConnected:  config.PeeringConfig.GetInt(config.CfgPeeringMaxPeers),
			AcceptAnyPeer: config.PeeringConfig.GetBool(config.CfgPeeringAcceptAnyConnection),
		}, peers...)
	})
	return manager
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// init peering manager
	Manager()

	// register log event handlers
	configureManagerEventHandlers()

	// react to peer config changes
	configurePeerConfigWatcher()
}

func configureManagerEventHandlers() {
	manager.Events.PeerHandshakingOutgoing.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("handshaking with %s...", p.ID)
	}))

	manager.Events.PeerHandshakingIncoming.Attach(events.NewClosure(func(addr string) {
		log.Infof("handshaking with incoming connection from %s", addr)
	}))

	manager.Events.PeerConnected.Attach(events.NewClosure(func(p *peer.Peer) {
		var autopeeringMeta string
		if p.Autopeering != nil {
			autopeeringMeta = fmt.Sprintf(" [autopeered %s]", p.Autopeering.ID())
		}
		featureSetMeta := fmt.Sprintf(" [feature set(s): %s]", strings.Join(p.Protocol.SupportedFeatureSets(), ","))
		var aliasMeta string
		if len(p.InitAddress.Alias) > 0 {
			aliasMeta = fmt.Sprintf(" [alias: %s]", p.InitAddress.Alias)
		}
		log.Infof("connected with %s%s%s%s", p.ID, aliasMeta, featureSetMeta, autopeeringMeta)
	}))

	manager.Events.PeerMovedIntoReconnectPool.Attach(events.NewClosure(func(addr *iputils.OriginAddress) {
		log.Infof("moved %s into reconnect pool", addr.String())
	}))

	manager.Events.PeerMovedFromConnectedToReconnectPool.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("moved disconnected %s into the reconnect pool", p.ID)
	}))

	manager.Events.PeerDisconnected.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("disconnected %s", p.ID)
	}))

	manager.Events.AutopeeredPeerHandshaking.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("handshaking with autopeered peer %s / %s", p.ID, p.Autopeering.ID())
	}))

	manager.Events.Reconnecting.Attach(events.NewClosure(func(count int32) {
		log.Infof("trying to connect to %d peers", count)
	}))

	manager.Events.ReconnectRemovedAlreadyConnected.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("removed already connected peer %s from reconnect pool", p.ID)
	}))

	manager.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Warnf("error %s", err)
	}))
}

func run(_ *node.Plugin) {

	runConfigWatcher()

	peeringBindAddr := config.NodeConfig.GetString(config.CfgNetGossipBindAddress)
	daemon.BackgroundWorker("Peering Server", func(shutdownSignal <-chan struct{}) {
		log.Infof("Peering Server (%s) ...", peeringBindAddr)
		go func() {
			// start listening for incoming connections
			if err := manager.Listen(); err != nil {
				log.Fatal(err)
			}
		}()
		log.Infof("Peering Server (%s) ... done", peeringBindAddr)
		<-shutdownSignal
		log.Info("Stopping Peering Server ...")
		manager.Shutdown()
		log.Info("Stopping Peering Server ... done")
	}, shutdown.PriorityPeeringTCPServer)

	// get reconnect config
	intervalSec := config.NodeConfig.GetInt(config.CfgNetGossipReconnectAttemptIntervalSeconds)
	reconnectAttemptInterval := time.Duration(intervalSec) * time.Second

	daemon.BackgroundWorker("Peering Reconnect", func(shutdownSignal <-chan struct{}) {

		// do first "reconnect" to initial peers
		go manager.Reconnect()

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping Reconnecter")
				log.Info("Stopping Reconnecter ... done")
				return
			case <-time.After(reconnectAttemptInterval):
				manager.Reconnect()
			}
		}
	}, shutdown.PriorityPeerReconnecter)

	if config.NodeConfig.GetInt(config.CfgNetAutopeeringMaxDroppedPacketsPercentage) != 0 {
		// create a background worker that checks for staled autopeers every minute
		daemon.BackgroundWorker("Peering StaleCheck", func(shutdownSignal <-chan struct{}) {

			checkStaledPeers := func() {
				peerIDsToRemove := make(map[string]struct{})

				Manager().ForAllConnected(func(p *peer.Peer) bool {
					staled, droppedPercentage := p.CheckStaledAutopeer(config.NodeConfig.GetInt(config.CfgNetAutopeeringMaxDroppedPacketsPercentage))
					if !staled {
						// peer is healthy
						return true
					}

					// peer is connected via autopeering and is staled.
					// it's better to drop the connection and free the slots for other peers.
					peerIDsToRemove[p.ID] = struct{}{}

					log.Infof("dropping autopeered neighbor %s / %s because %0.2f%% of the messages in the last minute were dropped", p.Autopeering.Address(), p.Autopeering.ID(), droppedPercentage)
					return true
				})

				for peerIDToRemove := range peerIDsToRemove {
					Manager().Remove(peerIDToRemove)
				}
			}

			timeutil.Ticker(checkStaledPeers, 60*time.Second, shutdownSignal)
		}, shutdown.PriorityPeerReconnecter)
	}
}
