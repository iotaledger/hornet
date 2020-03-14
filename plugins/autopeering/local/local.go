package local

import (
	"crypto/ed25519"
	"encoding/base64"
	"net"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"

	"github.com/gohornet/hornet/packages/autopeering/services"
	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/tangle"
)

var (
	instance *peer.Local
	once     sync.Once
)

func configureLocal() *peer.Local {
	log := logger.NewLogger("Local")

	var externalIP net.IP
	if str := config.NodeConfig.GetString(config.CfgNetAutopeeringExternalAddr); strings.ToLower(str) == "auto" {
		log.Info("Querying external IP ...")
		ip, err := netutil.GetPublicIP(config.NodeConfig.GetBool(config.CfgNetPreferIPv6))
		if err != nil {
			log.Fatalf("Error querying external IP: %s", err)
		}
		log.Infof("External IP queried: address=%s", ip.String())
		externalIP = ip
	} else {
		externalIP = net.ParseIP(str)
		if externalIP == nil {
			log.Fatalf("Invalid IP address (%s): %s", config.CfgNetAutopeeringExternalAddr, str)
		}
	}

	if !externalIP.IsGlobalUnicast() {
		log.Fatalf("IP is not a global unicast address: %s", externalIP.String())
	}

	_, peeringPort, err := net.SplitHostPort(config.NodeConfig.GetString(config.CfgNetAutopeeringBindAddr))
	if err != nil {
		log.Fatalf("autopeering bind address is invalid: %s", err)
	}

	// announce the peering service
	ownServices := service.New()
	ownServices.Update(service.PeeringKey, "udp", net.JoinHostPort(externalIP.String(), peeringPort))
	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		_, gossipBindAddrPort, err := net.SplitHostPort(config.NodeConfig.GetString(config.CfgNetGossipBindAddress))
		if err != nil {
			log.Fatalf("gossip bind address is invalid: %s", err)
		}
		gossipBindAddr := net.JoinHostPort(externalIP.String(), gossipBindAddrPort)
		ownServices.Update(services.GossipServiceKey(), "tcp", gossipBindAddr)
	}

	// set the private key from the seed provided in the config
	var seed [][]byte
	if str := config.NodeConfig.GetString(config.CfgNetAutopeeringSeed); str != "" {
		bytes, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			log.Fatalf("Invalid %s: %s", config.CfgNetAutopeeringSeed, err)
		}
		if l := len(bytes); l != ed25519.SeedSize {
			log.Fatalf("Invalid %s length: %d, need %d", config.CfgNetAutopeeringSeed, l, ed25519.SeedSize)
		}
		seed = append(seed, bytes)
	}

	db, err := database.Get(tangle.DBPrefixAutopeering, database.GetHornetBadgerInstance())
	if err != nil {
		log.Fatalf("Unable to create autopeering database: %s", err)
	}

	peerDB, err := peer.NewDB(db)
	if err != nil {
		log.Fatalf("Unable to create autopeering database: %s", err)
	}
	local, err := peer.NewLocal(ownServices, peerDB, seed...)
	if err != nil {
		log.Fatalf("Error creating local: %s", err)
	}
	log.Infof("Initialized local: %v", local)

	return local
}

func GetInstance() *peer.Local {
	once.Do(func() { instance = configureLocal() })
	return instance
}
