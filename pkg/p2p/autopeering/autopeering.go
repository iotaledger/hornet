package autopeering

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p/core/crypto"
	peer2 "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mr-tron/base58/base58"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/autopeering/server"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/iputils"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/netutil"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
)

const (
	// Version of the discovery protocol.
	protocolVersion = 1
	// ProtocolCode is the protocol code for autopeering within a multi address.
	ProtocolCode = 1337
	// the min size of a base58 encoded public key.
	autopeeringMinPubKeyBase58Size = 42
	// the max size of a base58 encoded public key.
	autopeeringMaxPubKeyBase58Size = 44
)

var (
	// ErrInvalidMultiAddrPubKeyAutopeering gets returned when a public key in a multi address autopeering protocol path is invalid.
	ErrInvalidMultiAddrPubKeyAutopeering = errors.New("invalid multi address autopeering public key")
	// ErrMultiAddrNoHost gets returned if a multi address does not contain any host, meaning it neither has a /ip4, /ip6 or /dns portion.
	ErrMultiAddrNoHost = errors.New("multi address contains no host")
)

// RegisterAutopeeringProtocolInMultiAddresses registers the autopeering protocol for multi addresses.
// The 'autopeering' protocol value is the base58 encoded ed25519 public key and must be always 44 in length.
func RegisterAutopeeringProtocolInMultiAddresses() error {
	autopeeringProto := multiaddr.Protocol{
		Name:       "autopeering",
		Code:       ProtocolCode,
		VCode:      multiaddr.CodeToVarint(ProtocolCode),
		Size:       multiaddr.LengthPrefixedVarSize,
		Path:       false,
		Transcoder: multiaddr.NewTranscoderFromFunctions(protoStringToBytes, protoBytesToString, protoValidBytes),
	}
	// inject autopeering protocol into multi address
	return multiaddr.AddProtocol(autopeeringProto)
}

func protoStringToBytes(s string) ([]byte, error) {
	if len(s) < autopeeringMinPubKeyBase58Size || len(s) > autopeeringMaxPubKeyBase58Size {
		return nil, fmt.Errorf("%w: wrong length (str to bytes), is %d (wanted %d-%d)", ErrInvalidMultiAddrPubKeyAutopeering, len(s), autopeeringMinPubKeyBase58Size, autopeeringMaxPubKeyBase58Size)
	}
	base58PubKey, err := base58.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("unable to base58 decode autopeering public key: %w", err)
	}

	return base58PubKey, nil
}

func protoBytesToString(bytes []byte) (string, error) {
	if len(bytes) != ed25519.PublicKeySize {
		return "", fmt.Errorf("%w: wrong length (bytes to str), is %d (wanted %d)", ErrInvalidMultiAddrPubKeyAutopeering, len(bytes), ed25519.PublicKeySize)
	}

	return base58.Encode(bytes), nil
}

func protoValidBytes(bytes []byte) error {
	if len(bytes) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: wrong length (bytes to str), is %d (wanted %d)", ErrInvalidMultiAddrPubKeyAutopeering, len(bytes), ed25519.PublicKeySize)
	}

	return nil
}

// ExtractPubKeyFromMultiAddr extracts an ed25519 public key from the autopeering protocol portion of a multi address.
func ExtractPubKeyFromMultiAddr(multiAddr multiaddr.Multiaddr) (*ed25519.PublicKey, error) {
	base58PubKey, err := multiAddr.ValueForProtocol(ProtocolCode)
	if err != nil {
		return nil, fmt.Errorf("unable to extract base58 public key from multi address: %w", err)
	}

	pubKeyBytes, err := base58.Decode(base58PubKey)
	if err != nil {
		return nil, fmt.Errorf("unable to decode public key from multi address: %w", err)
	}

	pubKey, _, err := ed25519.PublicKeyFromBytes(pubKeyBytes)
	if err != nil {
		return nil, err
	}

	return &pubKey, nil
}

// ExtractHostAndPortFromMultiAddr extracts the host and port from a multi address.
func ExtractHostAndPortFromMultiAddr(multiAddr multiaddr.Multiaddr, portProtoCode int) (string, int, error) {
	portStr, err := multiAddr.ValueForProtocol(portProtoCode)
	if err != nil {
		return "", 0, fmt.Errorf("unable to parse port for proto code %d: %w", portProtoCode, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("unable to parse port as number: %w", err)
	}

	ip4Addr, err := multiAddr.ValueForProtocol(multiaddr.P_IP4)
	if err == nil {
		return ip4Addr, port, nil
	}

	ip6Addr, err := multiAddr.ValueForProtocol(multiaddr.P_IP6)
	if err == nil {
		return ip6Addr, port, nil
	}

	hostName, err := multiAddr.ValueForProtocol(multiaddr.P_DNS)
	if err == nil {
		return hostName, port, nil
	}

	return "", 0, ErrMultiAddrNoHost
}

// MultiAddrFromPeeringService extracts a multi address from a hive peer's service.
func MultiAddrFromPeeringService(peer *peer.Peer, serviceKey service.Key) (multiaddr.Multiaddr, error) {
	var maBuilder strings.Builder
	peeringService := peer.Services().Get(serviceKey)
	if ipv4 := peer.IP().To4(); ipv4 != nil {
		maBuilder.WriteString("/ip4/")
	} else {
		maBuilder.WriteString("/ip6/")
	}

	maBuilder.WriteString(peer.IP().String())

	maBuilder.WriteString("/tcp/")
	maBuilder.WriteString(strconv.Itoa(peeringService.Port()))

	ma, err := multiaddr.NewMultiaddr(maBuilder.String())
	if err != nil {
		return nil, fmt.Errorf("unable to build multi string from hive peer data: %w", err)
	}

	return ma, nil
}

// HivePeerToAddrInfo converts data from a hive autopeering peer to an AddrInfo containing a MultiAddr to the peer's peering port.
func HivePeerToAddrInfo(peer *peer.Peer, serviceKey service.Key) (*peer2.AddrInfo, error) {
	libp2pID, err := ConvertHivePubKeyToPeerID(peer.PublicKey())
	if err != nil {
		return nil, err
	}

	ma, err := MultiAddrFromPeeringService(peer, serviceKey)
	if err != nil {
		return nil, err
	}

	return &peer2.AddrInfo{ID: libp2pID, Addrs: []multiaddr.Multiaddr{ma}}, nil
}

// HivePeerToPeerID converts data from a hive autopeering peer to a libp2p peer ID.
func HivePeerToPeerID(peer *peer.Peer) (peer2.ID, error) {
	libp2pID, err := ConvertHivePubKeyToPeerID(peer.PublicKey())
	if err != nil {
		return "", err
	}

	return libp2pID, nil
}

type LogF func(template string, args ...interface{})

// ConvertHivePubKeyToPeerIDOrLog converts a hive ed25519 public key from the hive package to a libp2p peer ID.
// if it fails it logs a warning with the error instead.
func ConvertHivePubKeyToPeerIDOrLog(hivePubKey ed25519.PublicKey, log LogF) *peer2.ID {
	peerID, err := ConvertHivePubKeyToPeerID(hivePubKey)
	if err != nil {
		log(err.Error())

		return nil
	}

	return &peerID
}

// ConvertHivePubKeyToPeerID converts a hive ed25519 public key from the hive package to a libp2p peer ID.
func ConvertHivePubKeyToPeerID(hivePubKey ed25519.PublicKey) (peer2.ID, error) {
	libp2pPubKey, err := crypto.UnmarshalEd25519PublicKey(hivePubKey[:])
	if err != nil {
		return "", err
	}

	return peer2.IDFromPublicKey(libp2pPubKey)
}

// ConvertPeerIDToHiveIdentityOrLog converts a libp2p peer ID to a hive identity.
// if it fails it logs a warning with the error instead.
func ConvertPeerIDToHiveIdentityOrLog(peer *p2p.Peer, log LogF) *identity.Identity {
	id, err := ConvertPeerIDToHiveIdentity(peer)
	if err != nil {
		log(err.Error())

		return nil
	}

	return id
}

// ConvertPeerIDToHiveIdentity converts a libp2p peer ID to a hive identity.
func ConvertPeerIDToHiveIdentity(peer *p2p.Peer) (*identity.Identity, error) {
	pubKey, err := peer.ID.ExtractPublicKey()
	if err != nil {
		return nil, fmt.Errorf("unable to extract public key from peer ID: %w", err)
	}
	rawPubKeyBytes, err := pubKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("unable to convert extracted public key to bytes: %w", err)
	}
	hivePubKey, _, err := ed25519.PublicKeyFromBytes(rawPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to convert raw public key bytes to hive public key: %w", err)
	}

	return identity.New(hivePubKey), nil
}

// ConvertLibP2PPrivateKeyToHive converts a libp2p private key to a hive private key.
func ConvertLibP2PPrivateKeyToHive(key *crypto.Ed25519PrivateKey) (*ed25519.PrivateKey, error) {
	privKeyBytes, err := key.Raw()
	if err != nil {
		return nil, err
	}

	hivePrivKey, _, err := ed25519.PrivateKeyFromBytes(privKeyBytes)
	if err != nil {
		return nil, err
	}

	return &hivePrivKey, nil
}

// parses an entry node multi address string.
// example: /ip4/127.0.0.1/udp/14626/autopeering/HmKTkSd9F6nnERBvVbr55FvL1hM5WfcLvsc9bc3hWxWc
func parseEntryNode(entryNodeMultiAddrStr string, preferIPv6 bool) (entryNode *peer.Peer, err error) {
	if entryNodeMultiAddrStr == "" {
		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	entryNodeMultiAddr, err := multiaddr.NewMultiaddr(entryNodeMultiAddrStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse entry node multi address: %w", err)
	}

	pubKey, err := ExtractPubKeyFromMultiAddr(entryNodeMultiAddr)
	if err != nil {
		return nil, err
	}

	host, port, err := ExtractHostAndPortFromMultiAddr(entryNodeMultiAddr, multiaddr.P_UDP)
	if err != nil {
		return nil, err
	}

	ipAddresses, err := iputils.GetIPAddressesFromHost(host)
	if err != nil {
		return nil, fmt.Errorf("unable to look up IP addresses for %s: %w", host, err)
	}

	services := service.New()
	services.Update(service.PeeringKey, "udp", port)

	ip := ipAddresses.GetPreferredAddress(preferIPv6)

	return peer.NewPeer(identity.New(*pubKey), ip, services), nil
}

type Manager struct {
	// the logger used to log events.
	*logger.WrappedLogger

	// bindAddress is the bind address for autopeering.
	bindAddress string
	// entryNodes are the entry nodes for autopeering.
	entryNodes []string
	// preferIPv6 indicates whether to prefer IPv6 over IPv4.
	preferIPv6 bool
	// p2pServiceKey is the service key used for the p2pService.
	p2pServiceKey service.Key
	// localPeerContainer is the container for the local autopeering peer and database.
	localPeerContainer *LocalPeerContainer
	// discoveryProtocol is the peer discovery protocol.
	discoveryProtocol *discover.Protocol
	// selectionProtocol is the peer selection protocol.
	selectionProtocol *selection.Protocol
}

func NewManager(log *logger.Logger, bindAddress string, entryNodes []string, preferIPv6 bool, p2pServiceKey service.Key) *Manager {

	return &Manager{
		WrappedLogger:      logger.NewWrappedLogger(log),
		bindAddress:        bindAddress,
		entryNodes:         entryNodes,
		preferIPv6:         preferIPv6,
		p2pServiceKey:      p2pServiceKey,
		localPeerContainer: nil,
		discoveryProtocol:  nil,
		selectionProtocol:  nil,
	}
}

// P2PServiceKey is the peering service key.
func (a *Manager) P2PServiceKey() service.Key {
	return a.p2pServiceKey
}

// LocalPeerContainer returns the container for the local autopeering peer and database.
func (a *Manager) LocalPeerContainer() *LocalPeerContainer {
	return a.localPeerContainer
}

// Selection returns the peer selection protocol.
func (a *Manager) Selection() *selection.Protocol {
	return a.selectionProtocol
}

// Discovery returns the peer discovery protocol.
func (a *Manager) Discovery() *discover.Protocol {
	return a.discoveryProtocol
}

func (a *Manager) Init(localPeerContainer *LocalPeerContainer, initSelection bool) {

	parseEntryNodes := func(entryNodesString []string, preferIPv6 bool) (result []*peer.Peer, err error) {
		for _, entryNodeDefinition := range entryNodesString {
			entryNode, err := parseEntryNode(entryNodeDefinition, preferIPv6)
			if err != nil {
				a.LogWarnf("invalid entry node; ignoring: %s, error: %s", entryNodeDefinition, err)

				continue
			}
			result = append(result, entryNode)
		}

		if len(result) == 0 {
			return nil, errors.New("no valid entry nodes found")
		}

		return result, nil
	}

	entryNodes, err := parseEntryNodes(a.entryNodes, a.preferIPv6)
	if err != nil {
		a.LogWarn(err)
	}

	a.localPeerContainer = localPeerContainer

	gossipServiceKeyHash := fnv.New32a()
	gossipServiceKeyHash.Write([]byte(a.p2pServiceKey))
	networkID := gossipServiceKeyHash.Sum32()

	a.discoveryProtocol = discover.New(localPeerContainer.Local(), protocolVersion, networkID, discover.Logger(a.LoggerNamed("disc")), discover.MasterPeers(entryNodes))

	if !initSelection {
		return
	}

	isValidPeer := func(p *peer.Peer) bool {
		p2pPeering := p.Services().Get(a.p2pServiceKey)
		if p2pPeering == nil {
			return false
		}

		if p2pPeering.Network() != "tcp" || !netutil.IsValidPort(p2pPeering.Port()) {
			return false
		}

		return true
	}

	a.selectionProtocol = selection.New(
		localPeerContainer.Local(),
		a.discoveryProtocol,
		selection.Logger(a.LoggerNamed("sel")),
		selection.NeighborValidator(selection.ValidatorFunc(isValidPeer)),
		selection.NeighborBlockDuration(0), // disable neighbor block duration (we manually block neighbors)
	)
}

func (a *Manager) Run(ctx context.Context) {
	a.LogInfo("\n\nWARNING: The autopeering plugin will disclose your public IP address to possibly all nodes and entry points. Please disable this plugin if you do not want this to happen!\n")

	lPeer := a.localPeerContainer.Local()
	peering := lPeer.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), a.bindAddress)
	if err != nil {
		a.LogFatalfAndExit("error resolving %s: %s", a.bindAddress, err)
	}

	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		a.LogFatalfAndExit("error listening: %s", err)
	}

	handlers := []server.Handler{a.discoveryProtocol}
	if a.selectionProtocol != nil {
		handlers = append(handlers, a.selectionProtocol)
	}

	// start a server doing discovery and peering
	srv := server.Serve(lPeer, conn, a.LoggerNamed("srv"), handlers...)

	// start the discovery on that connection
	a.discoveryProtocol.Start(srv)

	if a.selectionProtocol != nil {
		// start the peering on that connection
		a.selectionProtocol.Start(srv)
	}

	a.LogInfof("started: Address=%s/%s PublicKey=%s", localAddr.String(), localAddr.Network(), lPeer.PublicKey().String())

	<-ctx.Done()
	a.LogInfo("Stopping Autopeering ...")

	if a.selectionProtocol != nil {
		a.selectionProtocol.Close()
	}
	a.discoveryProtocol.Close()

	// underlying connection is closed by the server
	srv.Close()

	if err := a.localPeerContainer.Close(); err != nil {
		a.LogErrorf("error closing peer database: %s", err)
	}

	a.LogInfo("Stopping Autopeering ... done")
}
