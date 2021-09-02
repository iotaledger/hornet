package autopeering

import (
	"fmt"
	"hash/fnv"
	"net"

	"github.com/pkg/errors"

	"github.com/multiformats/go-multiaddr"

	"github.com/gohornet/hornet/pkg/p2p/autopeering"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/netutil"
)

const (
	protocolVersion = 1
)

var (
	// discoveryProtocol is the peer discovery protocol.
	discoveryProtocol *discover.Protocol
	// selectionProtocol is the peer selection protocol.
	selectionProtocol *selection.Protocol
)

func configureAutopeering(localPeerContainer *autopeering.LocalPeerContainer) {
	entryNodes, err := parseEntryNodes()
	if err != nil {
		Plugin.LogWarn(err)
	}

	gossipServiceKeyHash := fnv.New32a()
	gossipServiceKeyHash.Write([]byte(p2pServiceKey()))
	networkID := gossipServiceKeyHash.Sum32()

	discoveryProtocol = discover.New(localPeerContainer.Local(), protocolVersion, networkID, discover.Logger(Plugin.Logger().Named("disc")), discover.MasterPeers(entryNodes))

	// only enable peer selection when the peering plugin is enabled
	if deps.Manager == nil {
		return
	}

	isValidPeer := func(p *peer.Peer) bool {
		p2pPeering := p.Services().Get(p2pServiceKey())
		if p2pPeering == nil {
			return false
		}

		if p2pPeering.Network() != "tcp" || !netutil.IsValidPort(p2pPeering.Port()) {
			return false
		}
		return true
	}

	neighborValidator := selection.NeighborValidator(selection.ValidatorFunc(isValidPeer))
	selectionProtocol = selection.New(localPeerContainer.Local(), discoveryProtocol, selection.Logger(Plugin.Logger().Named("sel")), neighborValidator)
}

func start(localPeerContainer *autopeering.LocalPeerContainer, shutdownSignal <-chan struct{}) {
	Plugin.LogInfo("\n\nWARNING: The autopeering plugin will disclose your public IP address to possibly all nodes and entry points. Please disable this plugin if you do not want this to happen!\n")

	lPeer := localPeerContainer.Local()
	peering := lPeer.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), deps.NodeConfig.String(CfgNetAutopeeringBindAddr))
	if err != nil {
		Plugin.LogFatalf("error resolving %s: %s", deps.NodeConfig.String(CfgNetAutopeeringBindAddr), err)
	}

	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		Plugin.LogFatalf("error listening: %s", err)
	}

	handlers := []server.Handler{discoveryProtocol}
	if selectionProtocol != nil {
		handlers = append(handlers, selectionProtocol)
	}

	// start a server doing discovery and peering
	srv := server.Serve(lPeer, conn, Plugin.Logger().Named("srv"), handlers...)

	// start the discovery on that connection
	discoveryProtocol.Start(srv)

	if selectionProtocol != nil {
		// start the peering on that connection
		selectionProtocol.Start(srv)
	}

	Plugin.LogInfof("started: Address=%s/%s PublicKey=%s", localAddr.String(), localAddr.Network(), lPeer.PublicKey().String())

	<-shutdownSignal
	Plugin.LogInfo("Stopping Autopeering ...")

	if selectionProtocol != nil {
		selectionProtocol.Close()
	}
	discoveryProtocol.Close()

	// underlying connection is closed by the server
	srv.Close()

	if err := localPeerContainer.Close(); err != nil {
		Plugin.LogErrorf("error closing peer database: %s", err)
	}

	Plugin.LogInfo("Stopping Autopeering ... done")
}

// parses an entry node multi address string.
// example: /ip4/127.0.0.1/udp/14626/autopeering/HmKTkSd9F6nnERBvVbr55FvL1hM5WfcLvsc9bc3hWxWc
func parseEntryNode(entryNodeMultiAddrStr string) (entryNode *peer.Peer, err error) {
	if entryNodeMultiAddrStr == "" {
		return nil, nil
	}

	entryNodeMultiAddr, err := multiaddr.NewMultiaddr(entryNodeMultiAddrStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse entry node multi address: %w", err)
	}

	pubKey, err := autopeering.ExtractPubKeyFromMultiAddr(entryNodeMultiAddr)
	if err != nil {
		return nil, err
	}

	host, port, err := autopeering.ExtractHostAndPortFromMultiAddr(entryNodeMultiAddr, multiaddr.P_UDP)
	if err != nil {
		return nil, err
	}

	ipAddresses, err := iputils.GetIPAddressesFromHost(host)
	if err != nil {
		return nil, fmt.Errorf("unable to look up IP addresses for %s: %w", host, err)
	}

	services := service.New()
	services.Update(service.PeeringKey, "udp", port)

	ip := ipAddresses.GetPreferredAddress(deps.NodeConfig.Bool(CfgNetAutopeeringEntryNodesPreferIPv6))
	return peer.NewPeer(identity.New(*pubKey), ip, services), nil
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range deps.NodeConfig.Strings(CfgNetAutopeeringEntryNodes) {
		entryNode, err := parseEntryNode(entryNodeDefinition)
		if err != nil {
			Plugin.LogWarnf("invalid entry node; ignoring: %s, error: %s", entryNodeDefinition, err)
			continue
		}
		result = append(result, entryNode)
	}

	if len(result) == 0 {
		return nil, errors.New("no valid entry nodes found")
	}

	return result, nil
}
