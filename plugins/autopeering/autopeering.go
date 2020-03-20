package autopeering

import (
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"strings"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"

	"github.com/gohornet/hornet/packages/autopeering/services"
	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/plugins/autopeering/local"
)

const (
	protocolVersion = 1
)

var (
	// Discovery is the peer discovery protocol.
	Discovery *discover.Protocol
	// Selection is the peer selection protocol.
	Selection *selection.Protocol

	// ID is the node's autopeering ID
	ID string

	ErrParsingEntryNode = errors.New("can't parse entry node")

	log *logger.Logger
)

func configureAP() {
	entryNodes, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Entry node peers: %v", entryNodes)

	gossipServiceKeyHash := fnv.New32a()
	gossipServiceKeyHash.Write([]byte(services.GossipServiceKey()))
	networkID := gossipServiceKeyHash.Sum32()

	Discovery = discover.New(local.GetInstance(), protocolVersion, networkID, discover.Logger(log.Named("disc")), discover.MasterPeers(entryNodes))

	// enable peer selection only when gossip is enabled
	Selection = selection.New(local.GetInstance(), Discovery, selection.Logger(log.Named("sel")), selection.NeighborValidator(selection.ValidatorFunc(isValidNeighbor)))
}

// isValidNeighbor checks whether a peer is a valid neighbor.
func isValidNeighbor(p *peer.Peer) bool {
	// gossip must be supported
	gossipService := p.Services().Get(services.GossipServiceKey())
	if gossipService == nil {
		return false
	}

	// gossip service must be valid

	if gossipService.Network() != "tcp" || !netutil.IsValidPort(gossipService.Port()) {
		return false
	}
	return true
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping Autopeering ... done")

	lPeer := local.GetInstance()
	peering := lPeer.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), config.NodeConfig.GetString(config.CfgNetAutopeeringBindAddr))
	if err != nil {
		log.Fatalf("Error resolving %s: %v", config.CfgNetAutopeeringBindAddr, err)
	}

	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	handlers := []server.Handler{Discovery}
	if Selection != nil {
		handlers = append(handlers, Selection)
	}

	// start a server doing discovery and peering
	srv := server.Serve(lPeer, conn, log.Named("srv"), handlers...)
	defer srv.Close()

	// start the discovery on that connection
	Discovery.Start(srv)
	defer Discovery.Close()

	if Selection != nil {
		// start the peering on that connection
		Selection.Start(srv)
		defer Selection.Close()
	}

	ID = lPeer.ID().String()
	log.Infof("%s started: ID=%s Address=%s/%s PublicKey=%s", name, lPeer.ID(), localAddr.String(), localAddr.Network(), base64.StdEncoding.EncodeToString(lPeer.PublicKey()))

	<-shutdownSignal
	log.Info("Stopping Autopeering ...")
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range config.NodeConfig.GetStringSlice(config.CfgNetAutopeeringEntryNodes) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: entry node parts must be 2, is %d", ErrParsingEntryNode, len(parts))
		}

		pubKey, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid public key: %s", ErrParsingEntryNode, err)
		}

		entryAddr, err := iputils.ParseOriginAddress(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid entry node address %s", err, parts[1])
		}

		ipAddresses, err := iputils.GetIPAddressesFromHost(entryAddr.Addr)
		if err != nil {
			return nil, fmt.Errorf("%w: while handling %s", err, parts[1])
		}

		services := service.New()
		services.Update(service.PeeringKey, "udp", int(entryAddr.Port))

		ip := ipAddresses.GetPreferredAddress(config.NodeConfig.GetBool(config.CfgNetPreferIPv6))
		result = append(result, peer.NewPeer(pubKey, ip, services))
	}

	return result, nil
}
