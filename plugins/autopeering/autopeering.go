package autopeering

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"strings"

	"github.com/mr-tron/base58/base58"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/netutil"

	"github.com/gohornet/hornet/pkg/autopeering/services"
	"github.com/gohornet/hornet/pkg/config"
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
)

func configureAP(local *Local) {
	entryNodes, err := parseEntryNodes()
	if err != nil {
		log.Warn(err)
	}

	gossipServiceKeyHash := fnv.New32a()
	gossipServiceKeyHash.Write([]byte(services.GossipServiceKey()))
	networkID := gossipServiceKeyHash.Sum32()

	Discovery = discover.New(local.PeerLocal, protocolVersion, networkID, discover.Logger(log.Named("disc")), discover.MasterPeers(entryNodes))

	// enable peer selection only when gossip is enabled
	Selection = selection.New(local.PeerLocal, Discovery, selection.Logger(log.Named("sel")), selection.NeighborValidator(selection.ValidatorFunc(isValidPeer)))
}

// isValidPeer checks whether a peer is a valid peer.
func isValidPeer(p *peer.Peer) bool {
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

func start(local *Local, shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping Autopeering ... done")

	log.Info("\n\nWARNING: The autopeering plugin will disclose your public IP address to possibly all nodes and entry points. Please disable this plugin if you do not want this to happen!\n")

	lPeer := local.PeerLocal
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
	log.Infof("started: ID=%s Address=%s/%s PublicKey=%s", lPeer.ID(), localAddr.String(), localAddr.Network(), lPeer.PublicKey().String())

	<-shutdownSignal
	err = local.Close()
	if err != nil {
		log.Errorf("Error closing peer database: %v", err.Error())
	}
	log.Info("Stopping Autopeering ...")
}

func parseEntryNode(entryNodeDefinition string) (entryNode *peer.Peer, err error) {
	if entryNodeDefinition == "" {
		return nil, nil
	}

	parts := strings.Split(entryNodeDefinition, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: entry node parts must be 2, is %d", ErrParsingEntryNode, len(parts))
	}

	pubKey, err := base58.Decode(parts[0])
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

	publicKey, _, err := ed25519.PublicKeyFromBytes(pubKey)
	if err != nil {
		return nil, err
	}

	services := service.New()
	services.Update(service.PeeringKey, "udp", int(entryAddr.Port))

	ip := ipAddresses.GetPreferredAddress(config.NodeConfig.GetBool(config.CfgNetPreferIPv6))

	return peer.NewPeer(identity.New(publicKey), ip, services), nil
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range config.NodeConfig.GetStringSlice(config.CfgNetAutopeeringEntryNodes) {
		entryNode, err := parseEntryNode(entryNodeDefinition)
		if err != nil {
			log.Warnf("invalid entry node; ignoring: %v, error: %v", entryNodeDefinition, err)
			continue
		}
		result = append(result, entryNode)
	}

	if len(result) == 0 {
		return nil, errors.New("no valid entry nodes found")
	}

	return result, nil
}
