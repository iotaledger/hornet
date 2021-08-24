package autopeering

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"

	"github.com/multiformats/go-multiaddr"

	"github.com/gohornet/hornet/pkg/database"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/kvstore"
)

// LocalPeerContainer defines the container for the local autopeering peer.
type LocalPeerContainer struct {
	peerLocal *peer.Local
	store     kvstore.KVStore
	peerDB    *peer.DB
}

// Local returns the local hive.go peer from the container.
func (lpc *LocalPeerContainer) Local() *peer.Local {
	return lpc.peerLocal
}

func NewLocalPeerContainer(p2pServiceKey service.Key,
	seed []byte,
	p2pDatabasePath string,
	dbEngine database.Engine,
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

	store, err := database.StoreWithDefaultSettings(filepath.Join(p2pDatabasePath, "autopeering"), true, dbEngine)
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
