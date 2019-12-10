package gossip

import (
	"net"
	"strconv"
	"time"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/iputils"
	"github.com/gohornet/hornet/packages/logger"
	"github.com/gohornet/hornet/packages/network"
	"github.com/gohornet/hornet/packages/shutdown"
)

var reconnectLogger = logger.NewLogger("Reconnect Pool")

type NeighborConfig struct {
	Identity   string `json:"identity"`
	PreferIPv6 bool   `json:"prefer_ipv6"`
}

func configureReconnectPool() {
	neighborConfig := []NeighborConfig{}
	if err := parameter.NodeConfig.UnmarshalKey("network.neighbors", &neighborConfig); err != nil {
		panic(err)
	}
	for _, neighConf := range neighborConfig {
		if neighConf.Identity == "" {
			continue
		}

		originAddr, err := iputils.ParseOriginAddress(neighConf.Identity)
		if err != nil {
			panic(errors.Wrapf(err, "invalid neighbor address %s", neighConf.Identity))
		}
		originAddr.PreferIPv6 = neighConf.PreferIPv6

		// no need to lock the neighbors in the configure stage
		addNeighborToReconnectPool(originAddr)
	}

	// if a neighbor was handshaked, we move it from being in-flight to connected
	Events.NeighborHandshakeCompleted.Attach(events.NewClosure(func(neighbor *Neighbor, protocolVersion byte) {
		neighborsLock.Lock()
		moveNeighborToConnected(neighbor)
		neighborsLock.Unlock()
		gossipLogger.Infof("neighbor handshaked %s, using protocol version %d", neighbor.InitAddress.String(), protocolVersion)
	}))
}

func runReconnectPool() {
	// do the first "reconnect" attempt in the background to not block the server startup
	go func() {
		// we can't do a direct wakeup of the reconnecter goroutine, because daemons only run
		// after plugin run initialisation has finished
		go reconnect()
		spawnReconnecter()
	}()
}

func reconnect() {
	neighborsLock.Lock()
	newlyInFlight := make([]*Neighbor, 0)

	// try to lookup each address and if we fail to do so, keep the address in the reconnect pool
	for _, originAddr := range reconnectPool {
		host := originAddr.Addr
		neighborAddresses := iputils.NewNeighborIPAddresses()
		ip := net.ParseIP(host)
		if ip != nil {
			// host is an actual IP address
			neighborAddresses.Add(&iputils.IP{IP: ip})
			prefIP := neighborAddresses.GetPreferredAddress(originAddr.PreferIPv6)
			newlyInFlight = append(newlyInFlight, NewOutboundNeighbor(originAddr, prefIP, originAddr.Port, neighborAddresses))
			continue
		}

		ips, err := net.LookupHost(host)
		if err != nil {
			gossipLogger.Warningf("couldn't lookup ips for %s, error: %s", host, err.Error())
			continue
		}

		if len(ips) == 0 {
			gossipLogger.Warningf("no ips found for %s", host)
			continue
		}

		for _, ipAddr := range ips {
			ip = net.ParseIP(ipAddr)
			neighborAddresses.Add(&iputils.IP{IP: ip})
		}
		prefIP := neighborAddresses.GetPreferredAddress(false)
		newlyInFlight = append(newlyInFlight, NewOutboundNeighbor(originAddr, prefIP, originAddr.Port, neighborAddresses))
	}
	neighborsLock.Unlock()

	for _, neighbor := range newlyInFlight {

		neighborsLock.Lock()
		allowNeighborIdentity(neighbor)
		moveNeighborFromReconnectToInFlightPool(neighbor)
		neighborsLock.Unlock()

		if err := Connect(neighbor); err != nil {
			gossipLogger.Errorf("connection attempt to %s failed: %s", neighbor.InitAddress.String(), err.Error())
			neighborsLock.Lock()
			moveFromInFlightToReconnectPool(neighbor)
			neighborsLock.Unlock()
			continue
		}

		setupNeighborEventHandlers(neighbor)

		// kicks of the protocol by sending the handshake packet and then reading inbound data
		go neighbor.Protocol.Init()
	}
}

func spawnReconnecter() {
	intervalSec := parameter.NodeConfig.GetInt("network.reconnectAttemptIntervalSeconds")
	reconnectAttemptInterval := time.Duration(intervalSec) * time.Second

	reconnectLogger.Infof("reconnecter configured with %d seconds interval", intervalSec)
	daemon.BackgroundWorker("Reconnecter", func(shutdownSignal <-chan struct{}) {
		for {
			select {
			case <-shutdownSignal:
				return
			case <-reconnectPoolWakeup:
				reconnect()
			case <-time.After(reconnectAttemptInterval):
				reconnect()
			}
		}
	}, shutdown.ShutdownPriorityNeighborReconnecter)
}

func Connect(neighbor *Neighbor) error {
	addr := neighbor.PrimaryAddress.ToString() + ":" + strconv.Itoa(int(neighbor.InitAddress.Port))
	conn, err := net.DialTimeout("tcp", addr, time.Duration(5)*time.Second)
	if err != nil {
		return errors.Wrapf(NewConnectionFailureError(err), "error when connecting to neighbor %s", neighbor.Identity)
	}

	neighbor.SetProtocol(newProtocol(network.NewManagedConnection(conn)))
	return nil
}
