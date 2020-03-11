package gossip

import (
	"net"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"

	"github.com/gohornet/hornet/packages/autopeering/services"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
)

var (
	reconnectLogger *logger.Logger
)

func configureReconnectPool() {
	reconnectLogger = logger.NewLogger("Reconnect Pool")

	neighborConfig := []NeighborConfig{}
	if err := parameter.NeighborsConfig.UnmarshalKey("neighbors", &neighborConfig); err != nil {
		panic(err)
	}
	for _, neighConf := range neighborConfig {
		if neighConf.Identity == "" {
			continue
		}

		if neighConf.Identity == ExampleNeighborIdentity {
			// Ignore the example neighbor
			continue
		}

		originAddr, err := iputils.ParseOriginAddress(neighConf.Identity)
		if err != nil {
			panic(errors.Wrapf(err, "invalid neighbor address %s", neighConf.Identity))
		}
		originAddr.PreferIPv6 = neighConf.PreferIPv6
		originAddr.Alias = neighConf.Alias
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

	if len(reconnectPool) == 0 {
		neighborsLock.Unlock()
		return
	}

	gossipLogger.Infof("starting reconnect attempts to %d neighbors...", len(reconnectPool))

	// try to lookup each address and if we fail to do so, keep the address in the reconnect pool
next:
	for k, recNeigh := range reconnectPool {
		originAddr := recNeigh.OriginAddr
		neighborAddrs, err := iputils.GetIPAddressesFromHost(originAddr.Addr)
		if err != nil {
			gossipLogger.Warn(err.Error())
			continue
		}

		// cache ips
		recNeigh.mu.Lock()
		recNeigh.CachedIPs = neighborAddrs
		recNeigh.mu.Unlock()

		prefIP := neighborAddrs.GetPreferredAddress(originAddr.PreferIPv6)

		// don't do any new connection attempts, if the neighbor is already connected or in-flight
		for ip := range neighborAddrs.IPs {
			id := NewNeighborIdentity(ip.String(), originAddr.Port)
			if _, alreadyConnected := connectedNeighbors[id]; alreadyConnected {
				gossipLogger.Infof("neighbor %s already connected, removing it from reconnect pool...", connectedNeighbors[id].InitAddress.String())
				delete(reconnectPool, k)
				continue next
			}
			if _, alreadyInFlight := inFlightNeighbors[id]; alreadyInFlight {
				gossipLogger.Infof("neighbor %s already in-fight, removing it from reconnect pool...", connectedNeighbors[id].InitAddress.String())
				delete(reconnectPool, k)
				continue next
			}
		}
		neighbor := NewOutboundNeighbor(originAddr, prefIP, originAddr.Port, neighborAddrs)
		// inject autopeering info
		if recNeigh.Autopeering != nil {
			neighbor.Autopeering = recNeigh.Autopeering
		}
		newlyInFlight = append(newlyInFlight, neighbor)
	}
	neighborsLock.Unlock()

	for _, neighbor := range newlyInFlight {

		neighborsLock.Lock()
		allowNeighborIdentity(neighbor)
		moveNeighborFromReconnectToInFlightPool(neighbor)
		neighborsLock.Unlock()

		if neighbor.Autopeering != nil {
			gossipAddr := neighbor.Autopeering.Services().Get(services.GossipServiceKey()).String()
			gossipLogger.Infof("initiating connection to autopeered neighbor %s / %s", gossipAddr, neighbor.Autopeering.ID())
		}

		if err := Connect(neighbor); err != nil {
			gossipLogger.Warnf("connection attempt to %s failed: %s", neighbor.InitAddress.String(), err.Error())
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
			ips, err := iputils.GetIPAddressesFromHost(recNeigh.OriginAddr.Addr)
			if err != nil {
				gossipLogger.Warnf("can't check reconnect pool existence on %s, as IP lookups failed: %s", recNeigh.OriginAddr.String(), err.Error())
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
	conn, err := net.DialTimeout("tcp", addr, time.Duration(3)*time.Second)
	if err != nil {
		return errors.Wrapf(NewConnectionFailureError(err), "error when connecting to neighbor %s", neighbor.Identity)
	}

	neighbor.SetProtocol(newProtocol(network.NewManagedConnection(conn)))
	return nil
}
