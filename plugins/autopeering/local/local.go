package local

import (
	"crypto/ed25519"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/gohornet/hornet/packages/autopeering/services"
	"github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"
)

var (
	instance *peer.Local
	once     sync.Once
)

func configureLocal() *peer.Local {
	log := logger.NewLogger("Local")

	var externalIP net.IP
	if str := parameter.NodeConfig.GetString(CFG_EXTERNAL); strings.ToLower(str) == "auto" {
		log.Info("Querying external IP ...")
		ip, err := netutil.GetPublicIP(false)
		if err != nil {
			log.Fatalf("Error querying external IP: %s", err)
		}
		log.Infof("External IP queried: address=%s", ip.String())
		externalIP = ip
	} else {
		externalIP = net.ParseIP(str)
		if externalIP == nil {
			log.Fatalf("Invalid IP address (%s): %s", CFG_EXTERNAL, str)
		}
	}
	if !externalIP.IsGlobalUnicast() {
		log.Fatalf("IP is not a global unicast address: %s", externalIP.String())
	}

	peeringPort := strconv.Itoa(parameter.NodeConfig.GetInt(CFG_PORT))

	// announce the peering service
	ownServices := service.New()
	ownServices.Update(service.PeeringKey, "udp", net.JoinHostPort(externalIP.String(), peeringPort))
	if !parameter.NodeConfig.GetBool(CFG_ACT_AS_ENTRY_NODE) {
		ownServices.Update(services.GossipServiceKey(), "tcp", net.JoinHostPort(externalIP.String(), parameter.NodeConfig.GetString("network.port")))
	}

	// set the private key from the seed provided in the config
	var seed [][]byte
	if str := parameter.NodeConfig.GetString(CFG_SEED); str != "" {
		bytes, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			log.Fatalf("Invalid %s: %s", CFG_SEED, err)
		}
		if l := len(bytes); l != ed25519.SeedSize {
			log.Fatalf("Invalid %s length: %d, need %d", CFG_SEED, l, ed25519.SeedSize)
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
