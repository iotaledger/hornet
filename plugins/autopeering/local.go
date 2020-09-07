package autopeering

import (
	"net"
	"strconv"

	"github.com/mr-tron/base58/base58"
	"go.etcd.io/bbolt"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/pkg/autopeering/services"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

type Local struct {
	PeerLocal *peer.Local
	boltDb    *bbolt.DB
	peerDb    *peer.DB
}

func newLocal() *Local {
	log := logger.NewLogger("Local")

	var peeringIP net.IP

	// let the autopeering discover the IP
	if config.NodeConfig.GetBool(config.CfgNetPreferIPv6) {
		peeringIP = net.IPv6unspecified
	} else {
		peeringIP = net.IPv4zero
	}

	_, peeringPortStr, err := net.SplitHostPort(config.NodeConfig.GetString(config.CfgNetAutopeeringBindAddr))
	if err != nil {
		log.Fatalf("autopeering bind address is invalid: %s", err)
	}

	peeringPort, err := strconv.Atoi(peeringPortStr)
	if err != nil {
		log.Fatalf("Invalid autopeering port number: %s, Error: %s", peeringPortStr, err)
	}

	// announce the peering service
	ownServices := service.New()
	ownServices.Update(service.PeeringKey, "udp", peeringPort)

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		_, gossipBindAddrPortStr, err := net.SplitHostPort(config.NodeConfig.GetString(config.CfgNetGossipBindAddress))
		if err != nil {
			log.Fatalf("gossip bind address is invalid: %s", err)
		}

		gossipBindAddrPort, err := strconv.Atoi(gossipBindAddrPortStr)
		if err != nil {
			log.Fatalf("Invalid gossip port number: %s, Error: %s", gossipBindAddrPort, err)
		}

		ownServices.Update(services.GossipServiceKey(), "tcp", gossipBindAddrPort)
	}

	// set the private key from the seed provided in the config
	var seed [][]byte
	if str := config.NodeConfig.GetString(config.CfgNetAutopeeringSeed); str != "" {
		bytes, err := base58.Decode(str)
		if err != nil {
			log.Fatalf("Invalid %s: %s", config.CfgNetAutopeeringSeed, err)
		}
		if l := len(bytes); l != ed25519.SeedSize {
			log.Fatalf("Invalid %s length: %d, need %d", config.CfgNetAutopeeringSeed, l, ed25519.SeedSize)
		}
		seed = append(seed, bytes)
	}

	boltDb, err := bolt.CreateDB(config.NodeConfig.GetString(config.CfgDatabasePath), "peer.db")
	if err != nil {
		log.Fatalf("Unable to create autopeering database: %s", err)
	}

	peerDB, err := peer.NewDB(bolt.New(boltDb).WithRealm([]byte{tangle.StorePrefixAutopeering}))
	if err != nil {
		log.Fatalf("Unable to create autopeering database: %s", err)
	}

	local, err := peer.NewLocal(peeringIP, ownServices, peerDB, seed...)
	if err != nil {
		log.Fatalf("Error creating local: %s", err)
	}

	log.Infof("Initialized local: peer://%s@%s", local.PublicKey().String(), local.Address())

	return &Local{
		PeerLocal: local,
		boltDb:    boltDb,
		peerDb:    peerDB,
	}
}

func (l *Local) close() error {
	l.peerDb.Close()
	return l.boltDb.Close()
}
