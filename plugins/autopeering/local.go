package autopeering

import (
	"net"
	"strconv"

	"github.com/multiformats/go-multiaddr"
	"go.etcd.io/bbolt"

	"github.com/gohornet/hornet/core/p2p"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/logger"
)

// Local defines the local autopeering peer.
type Local struct {
	PeerLocal *peer.Local
	boltDB    *bbolt.DB
	peerDB    *peer.DB
}

func newLocal(seed []byte, p2pDatabasePath string) *Local {
	log := logger.NewLogger("Local")

	// let the autopeering discover the IP
	// TODO: is this really necessary?
	var peeringIP net.IP
	bindAddr := deps.NodeConfig.Strings(p2p.CfgP2PBindMultiAddresses)[0]
	multiAddrBindAddr, err := multiaddr.NewMultiaddr(bindAddr)
	if err != nil {
		log.Fatalf("unable to parse bind multi address %s", err)
		return nil
	}
	for _, proto := range multiAddrBindAddr.Protocols() {
		switch proto.Code {
		case multiaddr.P_IP4:
			peeringIP = net.IPv4zero
		case multiaddr.P_IP6:
			peeringIP = net.IPv6unspecified
		}
	}

	_, peeringPortStr, err := net.SplitHostPort(deps.NodeConfig.String(CfgNetAutopeeringBindAddr))
	if err != nil {
		log.Fatalf("autopeering bind address is invalid: %s", err)
	}

	peeringPort, err := strconv.Atoi(peeringPortStr)
	if err != nil {
		log.Fatalf("invalid autopeering port number: %s, Error: %s", peeringPortStr, err)
	}

	// announce the autopeering service
	ownServices := service.New()
	ownServices.Update(service.PeeringKey, "udp", peeringPort)

	if !deps.NodeConfig.Bool(CfgNetAutopeeringRunAsEntryNode) {
		libp2pBindPortStr, err := multiAddrBindAddr.ValueForProtocol(multiaddr.P_TCP)
		if err != nil {
			log.Fatalf("unable to extract libp2p bind port from multi address: %s", err)
		}

		libp2pBindPort, err := strconv.Atoi(libp2pBindPortStr)
		if err != nil {
			log.Fatalf("invalid libp2p bind port '%s': %s", libp2pBindPortStr, err)
		}

		ownServices.Update(p2pServiceKey(), "tcp", libp2pBindPort)
	}

	boltDB, err := bolt.CreateDB(p2pDatabasePath, "autopeering.db")
	if err != nil {
		log.Fatalf("unable to create autopeering database: %s", err)
	}

	// realm doesn't matter
	peerDB, err := peer.NewDB(bolt.New(boltDB))
	if err != nil {
		log.Fatalf("unable to create autopeering database: %s", err)
	}

	local, err := peer.NewLocal(peeringIP, ownServices, peerDB, [][]byte{seed}...)
	if err != nil {
		log.Fatalf("unable to create local autopeering peer instance: %s", err)
	}

	log.Infof("Initialized local autopeering: %s@%s", local.PublicKey(), local.Address())

	return &Local{PeerLocal: local, boltDB: boltDB, peerDB: peerDB}
}

func (l *Local) close() error {
	l.peerDB.Close()
	return l.boltDB.Close()
}
