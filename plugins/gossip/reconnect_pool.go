package gossip

import (
	"net"
	"strconv"
	"time"

	"github.com/pkg/errors"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/packages/iputils"
	"github.com/gohornet/hornet/packages/network"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	reconnectLogger *logger.Logger
)

type NeighborConfig struct {
	Identity   string `json:"identity"`
	PreferIPv6 bool   `json:"prefer_ipv6"`
}

func configureReconnectPool() {
	reconnectLogger = logger.NewLogger("Reconnect Pool", logger.LogLevel(parameter.NodeConfig.GetInt("node.logLevel")))

	neighborConfig := []NeighborConfig{}
	if err := parameter.NeighborsConfig.UnmarshalKey("network.neighbors", &neighborConfig); err != nil {
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
		addNeighborToReconnectPool(&reconnectneighbor{OriginAddr: originAddr})
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
	for _, recNeigh := range reconnectPool {
		originAddr := recNeigh.OriginAddr
		neighborAddrs, err := possibleIdentitiesFromNeighborAddress(originAddr)
		if err != nil {
			gossipLogger.Error(err.Error())
			continue
		}

		// cache ips
		recNeigh.mu.Lock()
		recNeigh.CachedIPs = neighborAddrs
		recNeigh.mu.Unlock()

		prefIP := neighborAddrs.GetPreferredAddress(originAddr.PreferIPv6)
		newlyInFlight = append(newlyInFlight, NewOutboundNeighbor(originAddr, prefIP, originAddr.Port, neighborAddrs))
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

func cleanReconnectPool(neighbor *Neighbor) {
	for key, recNeigh := range reconnectPool {
		recNeigh.mu.Lock()
		if recNeigh.CachedIPs == nil {
			ips, err := possibleIdentitiesFromNeighborAddress(recNeigh.OriginAddr)
			if err != nil {
				gossipLogger.Errorf("can't check reconnect pool existence on %s, as IP lookups failed: %s", recNeigh.OriginAddr.String(), err.Error())
				recNeigh.mu.Unlock()
				continue
			}
			recNeigh.CachedIPs = ips
		}
		for ip := range recNeigh.CachedIPs.IPs {
			if neighbor.Identity == NewNeighborIdentity(ip.String(), recNeigh.OriginAddr.Port) {
				// auto. set domain if it is empty by using the reconnect pool's entry
				if net.ParseIP(recNeigh.OriginAddr.Addr) == nil {
					neighbor.InitAddress.Addr = recNeigh.OriginAddr.Addr
				}

				// make an union of what the reconnect pool entry had
				neighbor.Addresses = recNeigh.CachedIPs.Union(neighbor.Addresses)

				// delete out of reconnect pool
				delete(reconnectPool, key)
				gossipLogger.Infof("removed %s from reconnect pool", key)
				break
			}
		}
		recNeigh.mu.Unlock()
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
				gossipLogger.Info("Stopping Reconnecter")
				gossipLogger.Info("Stopping Reconnecter ... done")
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
