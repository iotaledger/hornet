package autopeering

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p-core/crypto"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58/base58"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

const (
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

	hivePrivKey, err, _ := ed25519.PrivateKeyFromBytes(privKeyBytes)
	if err != nil {
		return nil, err
	}

	return &hivePrivKey, nil
}
