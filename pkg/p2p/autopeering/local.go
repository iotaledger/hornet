package autopeering

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/multiformats/go-multiaddr"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/database"
)

// LocalPeerContainer defines the container for the local autopeering peer.
type LocalPeerContainer struct {
	peerLocal *peer.Local
	store     kvstore.KVStore
	peerDB    *peer.DB
}

// Local returns the local hive.go peer from the container.
func (l *LocalPeerContainer) Local() *peer.Local {
	return l.peerLocal
}

// GetEntryNodeMultiAddress returns the multiaddress for the autopeering entry node.
func GetEntryNodeMultiAddress(local *peer.Local) (multiaddr.Multiaddr, error) {

	// example: /ip4/127.0.0.1/udp/14626/autopeering/HmKTkSd9F6nnERBvVbr55FvL1hM5WfcLvsc9bc3hWxWc
	localAddress := local.Address()

	var maBuilder strings.Builder
	if ipv4 := localAddress.IP.To4(); ipv4 != nil {
		maBuilder.WriteString("/ip4/")
	} else {
		maBuilder.WriteString("/ip6/")
	}
	maBuilder.WriteString(localAddress.IP.String())
	maBuilder.WriteString("/udp/")
	maBuilder.WriteString(strconv.Itoa(localAddress.Port))
	maBuilder.WriteString("/autopeering/")
	maBuilder.WriteString(local.PublicKey().String())

	return multiaddr.NewMultiaddr(maBuilder.String())
}

func NewLocalPeerContainer(p2pServiceKey service.Key,
	seed []byte,
	p2pDatabasePath string,
	dbEngine hivedb.Engine,
	p2pBindMultiAddresses []string,
	autopeeringBindAddr string,
	runAsEntryNode bool) (*LocalPeerContainer, error) {

	// let the autopeering discover the IP
	// TODO: is this really necessary?
	var peeringIP net.IP
	bindAddr := p2pBindMultiAddresses[0]
	multiAddrBindAddr, err := multiaddr.NewMultiaddr(bindAddr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse bind multi address %w", err)
	}

	for _, proto := range multiAddrBindAddr.Protocols() {
		switch proto.Code {
		case multiaddr.P_IP4:
			peeringIP = net.IPv4zero
		case multiaddr.P_IP6:
			peeringIP = net.IPv6unspecified
		}
	}

	_, peeringPortStr, err := net.SplitHostPort(autopeeringBindAddr)
	if err != nil {
		return nil, fmt.Errorf("autopeering bind address is invalid: %w", err)
	}

	peeringPort, err := strconv.Atoi(peeringPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid autopeering port number: %s, %w", peeringPortStr, err)
	}

	// announce the autopeering service
	ownServices := service.New()
	ownServices.Update(service.PeeringKey, "udp", peeringPort)

	if !runAsEntryNode {
		libp2pBindPortStr, err := multiAddrBindAddr.ValueForProtocol(multiaddr.P_TCP)
		if err != nil {
			return nil, fmt.Errorf("unable to extract libp2p bind port from multi address: %w", err)
		}

		libp2pBindPort, err := strconv.Atoi(libp2pBindPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid libp2p bind port '%s': %w", libp2pBindPortStr, err)
		}

		ownServices.Update(p2pServiceKey, "tcp", libp2pBindPort)
	}

	store, err := database.StoreWithDefaultSettings(filepath.Join(p2pDatabasePath, "autopeering"), true, dbEngine, database.AllowedEnginesDefault...)
	if err != nil {
		return nil, fmt.Errorf("unable to create autopeering database: %w", err)
	}

	// realm doesn't matter
	peerDB, err := peer.NewDB(store)
	if err != nil {
		return nil, fmt.Errorf("unable to create autopeering database: %w", err)
	}

	local, err := peer.NewLocal(peeringIP, ownServices, peerDB, [][]byte{seed}...)
	if err != nil {
		return nil, fmt.Errorf("unable to create local autopeering peer instance: %w", err)
	}

	return &LocalPeerContainer{peerLocal: local, store: store, peerDB: peerDB}, nil
}

func (l *LocalPeerContainer) Close() error {
	l.peerDB.Close()

	if err := l.store.Flush(); err != nil {
		return err
	}

	return l.store.Close()
}
