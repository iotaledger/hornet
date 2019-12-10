package gossip

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"sync"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/iputils"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
	"github.com/gohornet/hornet/packages/syncutils"
	"github.com/gohornet/hornet/plugins/gossip/neighbor"
)

var (
	// master lock protecting connected-, in-flight neighbors and the reconnect pool
	neighborsLock = sync.Mutex{}

	// holds neighbors which are fully connected and handshaked
	connectedNeighbors = make(map[string]*Neighbor)

	// in-flight: neighbors where we currently are trying to build up a connection to
	// and will commence a handshake
	inFlightNeighbors = make(map[string]*Neighbor)

	// holds IP/port or host/port combinations of neighbors which we want to be connected to
	reconnectPool       = make(map[string]*iputils.OriginAddress)
	reconnectPoolWakeup = make(chan struct{})

	// a set containing identities which are allowed to connect
	allowedIdentities = make(map[string]struct{})

	// a set containing IPs which are blacklisted
	// TODO: if there's multiple nodes from the same IP but one of them gets removed
	// it also blocks the other identities to connect
	hostsBlacklist     = make(map[string]struct{})
	hostsBlacklistLock = sync.Mutex{}

	handshakeFinalisationLock = syncutils.Mutex{}
	autoTetheringEnabled      bool
)

var (
	ErrNeighborSlotsFilled      = errors.New("neighbors slots filled")
	ErrNotMatchingMWM           = errors.New("used MWM doesn't match")
	ErrNotMatchingCooAddr       = errors.New("used coo addr doesn't match")
	ErrNotMatchingSrvSocketPort = errors.New("advertised server socket port doesn't match")
	ErrIdentityUnknown          = errors.New("neighbor identity is not known")
	ErrNeighborAlreadyConnected = errors.New("neighbor is already connected")
	ErrNeighborAlreadyKnown     = errors.New("neighbor is already known")
	// TODO: perhaps better naming
	ErrNoIPsFound = errors.New("didn't find any IPs")
)

func availableNeighborSlotsFilled() bool {
	// while this check is not thread-safe, initiated connections will be dropped
	// when their handshaking was done but already all neighbor slots are filled
	return len(connectedNeighbors) >= parameter.NodeConfig.GetInt("network.maxNeighbors")
}

func configureNeighbors() {
	autoTetheringEnabled = parameter.NodeConfig.GetBool("network.autoTetheringEnabled")

	Events.NeighborPutBackIntoReconnectPool.Attach(events.NewClosure(func(neighbor *Neighbor) {
		gossipLogger.Infof("added neighbor %s back into reconnect pool...", neighbor.InitAddress.String())
	}))

	Events.NeighborPutIntoConnectedPool.Attach(events.NewClosure(func(neighbor *Neighbor) {
		gossipLogger.Infof("neighbor %s is now connected", neighbor.InitAddress.String())
	}))

	Events.NeighborPutIntoInFlightPool.Attach(events.NewClosure(func(neighbor *Neighbor) {
		gossipLogger.Infof("connecting and initiating handshake for neighbor %s", neighbor.InitAddress.String())
	}))

	Events.NeighborPutIntoReconnectPool.Attach(events.NewClosure(func(originAddr *iputils.OriginAddress) {
		gossipLogger.Infof("added neighbor %s into reconnect pool for the first time", originAddr.String())
	}))

	daemon.BackgroundWorker("NeighborConnections", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		for _, neighbor := range connectedNeighbors {
			RemoveNeighbor(neighbor.Identity)
		}
		for _, neighbor := range inFlightNeighbors {
			RemoveNeighbor(neighbor.Identity)
		}

		gossipLogger.Info("Closing neighbor connections ... done")
	}, shutdown.ShutdownPriorityNeighbors)
}

type ConnectionOrigin byte

const (
	Inbound ConnectionOrigin = iota
	Outbound
)

type Neighbor struct {
	InitAddress *iputils.OriginAddress
	// The ip/port combination of the neighbor
	Identity string
	// The domain under which the neighbor was initially added
	Domain string
	// The address IP address under which the neighbor is connected
	PrimaryAddress *iputils.IP
	// The IP addresses which were looked up during neighbor initialisation
	Addresses *iputils.NeighborIPAddresses
	// The protocol instance under which this neighbor operates
	Protocol *protocol
	// Events on this neighbor
	Events neighborEvents
	// Metrics about the neighbor
	Metrics *neighbor.NeighborMetrics
	// Whether the connection for this neighbor was handled inbound or was created outbound
	ConnectionOrigin ConnectionOrigin
	// Whether to place this neighbor back into the reconnect pool when the connection is closed
	MoveBackToReconnectPool bool
	// The neighbors latest heartbeat message
	LatestHeartbeat *Heartbeat
}

// IdentityOrAddress gets the identity if set or the address otherwise.
func (n *Neighbor) IdentityOrAddress() string {
	if len(n.Identity) != 0 {
		return n.Identity
	}
	return n.PrimaryAddress.String()
}

func (n *Neighbor) SetProtocol(protocol *protocol) {
	n.Protocol = protocol
	protocol.Neighbor = n
}

func NewNeighborIdentity(ip string, port uint16) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func NewInboundNeighbor(remoteAddr net.Addr) *Neighbor {
	ip := net.ParseIP(remoteAddr.(*net.TCPAddr).IP.String())
	addresses := iputils.NewNeighborIPAddresses()
	primaryAddr := &iputils.IP{IP: ip}
	addresses.Add(primaryAddr)

	// InitAddress and Identity are set in finalizeHandshake
	return &Neighbor{
		Domain:         "",
		PrimaryAddress: primaryAddr,
		Addresses:      addresses,
		Events: neighborEvents{
			ProtocolConnectionEstablished: events.NewEvent(protocolCaller),
		},
		Metrics:          &neighbor.NeighborMetrics{},
		ConnectionOrigin: Inbound,
	}
}

func NewOutboundNeighbor(originAddr *iputils.OriginAddress, primaryAddr *iputils.IP, port uint16, addresses *iputils.NeighborIPAddresses) *Neighbor {
	return &Neighbor{
		InitAddress:    originAddr,
		Identity:       NewNeighborIdentity(primaryAddr.String(), port),
		PrimaryAddress: primaryAddr,
		Addresses:      addresses,
		Events: neighborEvents{
			ProtocolConnectionEstablished: events.NewEvent(protocolCaller),
		},
		Metrics:          &neighbor.NeighborMetrics{},
		ConnectionOrigin: Outbound,
	}
}

func addNeighborToReconnectPool(neighborAddr *iputils.OriginAddress) {
	if _, has := reconnectPool[neighborAddr.String()]; has {
		return
	}
	reconnectPool[neighborAddr.String()] = neighborAddr
	Events.NeighborPutIntoReconnectPool.Trigger(neighborAddr)
}

func moveNeighborFromReconnectToInFlightPool(neighbor *Neighbor) {
	// neighbors lock must be held by caller
	delete(reconnectPool, neighbor.InitAddress.String())
	inFlightNeighbors[neighbor.Identity] = neighbor
	Events.NeighborPutIntoInFlightPool.Trigger(neighbor)
}

func moveFromInFlightToReconnectPool(neighbor *Neighbor) {
	// neighbors lock must be held by caller
	delete(inFlightNeighbors, neighbor.Identity)
	reconnectPool[neighbor.InitAddress.String()] = neighbor.InitAddress
	Events.NeighborPutBackIntoReconnectPool.Trigger(neighbor)
}

func moveNeighborToConnected(neighbor *Neighbor) {
	// neighbors lock must be held by caller
	delete(inFlightNeighbors, neighbor.Identity)
	connectedNeighbors[neighbor.Identity] = neighbor

	// also delete any ongoing reconnect attempt
	delete(reconnectPool, neighbor.InitAddress.String())
	Events.NeighborPutIntoConnectedPool.Trigger(neighbor)
}

func moveNeighborFromConnectedToReconnectPool(neighbor *Neighbor) {
	if !neighbor.MoveBackToReconnectPool {
		return
	}

	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	connectedNeighbor, ok := connectedNeighbors[neighbor.Identity]
	if !ok && connectedNeighbor != neighbor {
		return
	}
	delete(connectedNeighbors, neighbor.Identity)
	reconnectPool[neighbor.InitAddress.String()] = neighbor.InitAddress
	Events.NeighborPutBackIntoReconnectPool.Trigger(neighbor)
}

func allowNeighborIdentity(neighbor *Neighbor) {
	for ip := range neighbor.Addresses.IPs {
		identity := NewNeighborIdentity(ip.String(), neighbor.InitAddress.Port)
		allowedIdentities[identity] = struct{}{}
		delete(hostsBlacklist, ip.String())
	}
}

func finalizeHandshake(protocol *protocol, handshake *Handshake) error {
	// make sure only one handshake finalisation process is ongoing at once
	handshakeFinalisationLock.Lock()
	defer handshakeFinalisationLock.Unlock()

	neighbor := protocol.Neighbor

	// drop the connection if in the meantime the available neighbor slots were filled
	if availableNeighborSlotsFilled() {
		return ErrNeighborSlotsFilled
	}

	// check whether same MWM is used
	if handshake.MWM != byte(ownMWM) {
		return errors.Wrapf(ErrNotMatchingMWM, "different MWM (%d instead of %d)", handshake.MWM, ownMWM)
	}

	// check whether the neighbor actually uses the same coordinator address
	if !bytes.Equal(ownByteEncodedCooAddress, handshake.ByteEncodedCooAddress) {
		return ErrNotMatchingCooAddr
	}

	// check whether we support the supported protocol versions by the neighbor
	version, err := handshake.CheckNeighborSupportedVersion()
	if err != nil {
		return errors.Wrapf(err, "protocol version %d is not supported", version)
	}

	switch neighbor.ConnectionOrigin {
	case Inbound:
		// set this neighbor's identity for the first time as we
		// now have the used server socket port information
		remoteIPStr := protocol.Conn.RemoteAddr().(*net.TCPAddr).IP.String()
		neighbor.Identity = NewNeighborIdentity(remoteIPStr, handshake.ServerSocketPort)
		neighbor.InitAddress = &iputils.OriginAddress{
			Addr: remoteIPStr,
			Port: handshake.ServerSocketPort,
		}
	case Outbound:
		expectedPort := neighbor.InitAddress.Port
		if handshake.ServerSocketPort != expectedPort {
			return errors.Wrapf(ErrNotMatchingSrvSocketPort, "expected %d as the server socket port but got %d", expectedPort, handshake.ServerSocketPort)
		}
	}

	// check whether the neighbor is already connected by checking each neighbors' IP addresses
	neighborsLock.Lock()
	for _, connectedNeighbor := range connectedNeighbors {
		// we need to loop through because the map holds pointer values
		for handshakingNeighborIP := range neighbor.Addresses.IPs {
			for ip := range connectedNeighbor.Addresses.IPs {
				if ip.String() == handshakingNeighborIP.String() &&
					connectedNeighbor.InitAddress.Port == neighbor.InitAddress.Port {
					neighborsLock.Unlock()
					return errors.Wrapf(ErrNeighborAlreadyConnected, neighbor.Identity)
				}
			}
		}
	}

	if !autoTetheringEnabled {
		if _, allowedToConnect := allowedIdentities[neighbor.Identity]; !allowedToConnect {
			hostsBlacklist[neighbor.PrimaryAddress.String()] = struct{}{}
			neighborsLock.Unlock()
			return errors.Wrapf(ErrIdentityUnknown, neighbor.Identity)
		}
	}
	neighborsLock.Unlock()

	protocol.Version = byte(version)
	protocol.ReceivedHandshake()
	return nil
}

func setupNeighborEventHandlers(neighbor *Neighbor) {

	// flag this neighbor to be put back into the reconnect pool.
	// this flag will be set to false if the neighbor is explicitly removed
	neighbor.MoveBackToReconnectPool = true

	// print protocol error log
	neighbor.Protocol.Events.Error.Attach(events.NewClosure(func(err error) {
		gossipLogger.Errorf("protocol error on neighbor %s: %s", neighbor.IdentityOrAddress(), err.Error())
	}))

	// connection error log
	neighbor.Protocol.Conn.Events.Error.Attach(events.NewClosure(func(err error) {
		gossipLogger.Errorf("connection error on neighbor %s: %s", neighbor.IdentityOrAddress(), err.Error())
	}))

	// automatically put the disconnected neighbor back into the reconnect pool
	// if not closed on purpose
	neighbor.Protocol.Conn.Events.Close.Attach(events.NewClosure(func() {
		gossipLogger.Infof("connection closed to %s", neighbor.IdentityOrAddress())
		moveNeighborFromConnectedToReconnectPool(neighbor)
	}))

	neighbor.Protocol.Events.HandshakeCompleted.Attach(events.NewClosure(func(protocolVersion byte) {

		neighborQueuesMutex.Lock()
		queue := newNeighborQueue(neighbor.Protocol)
		neighborQueues[neighbor.Identity] = queue
		neighborQueuesMutex.Unlock()

		// automatically remove the neighbor send queue if the connection gets closed
		closeNeighborQueueClosure := events.NewClosure(func() {
			neighborQueuesMutex.Lock()
			close(queue.disconnectChan)
			delete(neighborQueues, neighbor.Identity)
			neighborQueuesMutex.Unlock()
		})
		neighbor.Protocol.Conn.Events.Close.Attach(closeNeighborQueueClosure)
		startNeighborSendQueue(neighbor, queue)

		// register packet routing events
		receiveLegacyTransactionDataClosure := events.NewClosure(func(protocol *protocol, data []byte) {
			packetProcessorWorkerPool.Submit(protocol, data, PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP)
		})
		neighbor.Protocol.Events.ReceivedLegacyTransactionGossipData.Attach(receiveLegacyTransactionDataClosure)

		receiveTransactionDataClosure := events.NewClosure(func(protocol *protocol, data []byte) {
			packetProcessorWorkerPool.Submit(protocol, data, PROTOCOL_MSG_TYPE_TX_GOSSIP)
		})
		neighbor.Protocol.Events.ReceivedTransactionGossipData.Attach(receiveTransactionDataClosure)

		transactionRequestDataClosure := events.NewClosure(func(protocol *protocol, data []byte) {
			packetProcessorWorkerPool.Submit(protocol, data, PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP)
		})
		neighbor.Protocol.Events.ReceivedTransactionRequestGossipData.Attach(transactionRequestDataClosure)

		receiveMilestoneRequestClosure := events.NewClosure(func(protocol *protocol, data []byte) {
			packetProcessorWorkerPool.Submit(protocol, data, PROTOCOL_MSG_TYPE_MS_REQUEST)
		})
		neighbor.Protocol.Events.ReceivedMilestoneRequestData.Attach(receiveMilestoneRequestClosure)

		heartbeatClosure := events.NewClosure(func(protocol *protocol, data []byte) {
			// if we are receiving the first heartbeat message, we fire a "SendMilestoneRequest" call
			firstHeartbeat := neighbor.LatestHeartbeat == nil
			neighbor.LatestHeartbeat = HeartbeatFromBytes(data)
			if firstHeartbeat {
				SendMilestoneRequests(tangle.GetSolidMilestoneIndex(), tangle.GetLatestMilestoneIndex())
			}
		})
		neighbor.Protocol.Events.ReceivedHeartbeatData.Attach(heartbeatClosure)

		neighbor.Protocol.Conn.Events.Close.Attach(events.NewClosure(func() {
			neighbor.Protocol.Events.ReceivedLegacyTransactionGossipData.Detach(receiveLegacyTransactionDataClosure)
			neighbor.Protocol.Events.ReceivedTransactionGossipData.Detach(receiveTransactionDataClosure)
			neighbor.Protocol.Events.ReceivedMilestoneRequestData.Detach(receiveMilestoneRequestClosure)
			neighbor.Protocol.Events.ReceivedHeartbeatData.Detach(heartbeatClosure)
		}))

		Events.NeighborHandshakeCompleted.Trigger(neighbor, protocolVersion)
	}))

	neighbor.Events.ProtocolConnectionEstablished.Trigger(neighbor.Protocol)
}

func wakeupReconnectPool() {
	reconnectPoolWakeup <- struct{}{}
}

func AddNeighbor(neighborAddr string) error {
	originAddr, err := iputils.ParseOriginAddress(neighborAddr)
	if err != nil {
		return errors.Wrapf(err, "invalid neighbor address %s", neighborAddr)
	}

	// check whether the neighbor is already connected, in-flight or in the reconnect pool
	// given any of the IP addresses to which the neighbor address resolved to
	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	// check whether already in reconnect pool
	if _, exists := reconnectPool[neighborAddr]; exists {
		return errors.Wrapf(ErrNeighborAlreadyKnown, "%s is already known and in the reconnect pool", neighborAddr)
	}

	possibleIdentities, err := possibleIdentitiesFromNeighborAddress(originAddr)
	if err != nil {
		return err
	}
	for ip := range possibleIdentities.IPs {
		identity := NewNeighborIdentity(ip.String(), originAddr.Port)
		if _, exists := connectedNeighbors[identity]; exists {
			return errors.Wrapf(ErrNeighborAlreadyConnected, "%s is already connected via identity %s", neighborAddr, identity)
		}
		if _, exists := inFlightNeighbors[identity]; exists {
			return errors.Wrapf(ErrNeighborAlreadyKnown, "%s is already known and in-flight via %s", neighborAddr, identity)
		}
	}
	addNeighborToReconnectPool(originAddr)
	// force reconnect attempts now
	wakeupReconnectPool()
	return nil
}

func possibleIdentitiesFromNeighborAddress(originAddr *iputils.OriginAddress) (*iputils.NeighborIPAddresses, error) {
	possibleIdentities := iputils.NewNeighborIPAddresses()
	ip := net.ParseIP(originAddr.Addr)
	if ip != nil {
		possibleIdentities.Add(&iputils.IP{IP: ip})
		return possibleIdentities, nil
	}
	ips, err := net.LookupHost(originAddr.Addr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't lookup ips for %s, error: %s", originAddr.Addr, err.Error())
	}
	if len(ips) == 0 {
		return nil, errors.Wrapf(ErrNoIPsFound, "no ips found for %s", originAddr.Addr)
	}
	for _, ipAddr := range ips {
		possibleIdentities.Add(&iputils.IP{IP: net.ParseIP(ipAddr)})
	}
	return possibleIdentities, nil
}

func RemoveNeighbor(originIdentity string) error {
	originAddr, err := iputils.ParseOriginAddress(originIdentity)
	if err != nil {
		panic(errors.Wrapf(err, "invalid neighbor address %s", originIdentity))
	}
	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	// always remove the neighbor from the reconnect pool through its origin identity
	delete(reconnectPool, originIdentity)

	possibleIdentities, err := possibleIdentitiesFromNeighborAddress(originAddr)
	if err != nil {
		return err
	}

	// make sure the neighbor is removed by all its possible identities by going
	// through each resolved IP address from the lookup
	for ip := range possibleIdentities.IPs {
		identity := NewNeighborIdentity(ip.String(), originAddr.Port)

		// close the connection of the neighbor and remove it from the connected pool
		if neigh, exists := connectedNeighbors[identity]; exists {
			neigh.MoveBackToReconnectPool = false
			delete(connectedNeighbors, identity)
			neigh.Protocol.Conn.Close()
			Events.RemovedNeighbor.Trigger(neigh)
			// if the neighbor is in-flight, also close the connection and remove it from the pool
		} else if neigh, exists := inFlightNeighbors[identity]; exists {
			delete(inFlightNeighbors, identity)
			neigh.MoveBackToReconnectPool = false
			if neigh.Protocol != nil && neigh.Protocol.Conn != nil {
				neigh.Protocol.Conn.Close()
			}
			Events.RemovedNeighbor.Trigger(neigh)
		}

		// remove the neighbor from the reconnect pool and allowed identities
		// and add it to the blacklist
		delete(reconnectPool, identity)
		delete(allowedIdentities, identity)
		hostsBlacklist[ip.String()] = struct{}{}
	}

	// also remove the neighbor if the origin address matches:
	// this could happen if the DNS for the given neighbor updated and hence
	// just matching by IP/Port wouldn't render any neighbor to be removed
	for _, neigh := range connectedNeighbors {
		if originIdentity == neigh.InitAddress.String() {
			neigh.MoveBackToReconnectPool = false
			delete(connectedNeighbors, originIdentity)
			neigh.Protocol.Conn.Close()
			Events.RemovedNeighbor.Trigger(neigh)
			delete(reconnectPool, neigh.Identity)
			delete(allowedIdentities, neigh.Identity)
			hostsBlacklist[neigh.Identity] = struct{}{}
		}
	}

	return nil
}

func IsAddrBlacklisted(remoteAddr net.Addr) bool {
	tcpAddr, ok := remoteAddr.(*net.TCPAddr)
	if !ok {
		return false
	}
	hostsBlacklistLock.Lock()
	defer hostsBlacklistLock.Unlock()
	_, isBlacklisted := hostsBlacklist[tcpAddr.IP.String()]
	return isBlacklisted
}

type NeighborInfo struct {
	Neighbor                          *Neighbor `json:"-"`
	Address                           string    `json:"address"`
	Port                              uint16    `json:"port,omitempty"`
	Domain                            string    `json:"domain,omitempty"`
	NumberOfAllTransactions           uint32    `json:"numberOfAllTransactions"`
	NumberOfRandomTransactionRequests uint32    `json:"numberOfRandomTransactionRequests"`
	NumberOfNewTransactions           uint32    `json:"numberOfNewTransactions"`
	NumberOfInvalidTransactions       uint32    `json:"numberOfInvalidTransactions"`
	NumberOfStaleTransactions         uint32    `json:"numberOfStaleTransactions"`
	NumberOfSentTransactions          uint32    `json:"numberOfSentTransactions"`
	NumberOfDroppedSentPackets        uint32    `json:"numberOfDroppedSentPackets"`
	ConnectionType                    string    `json:"connectionType"`
	Connected                         bool      `json:"connected"`
}

func GetNeighbor(identifier string) (*Neighbor, bool) {
	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	neighbor, exists := connectedNeighbors[identifier]
	return neighbor, exists
}

func GetConnectedNeighbors() map[string]*Neighbor {
	neighborsLock.Lock()
	defer neighborsLock.Unlock()
	result := make(map[string]*Neighbor)
	for id, neighbor := range connectedNeighbors {
		result[id] = neighbor
	}
	return result
}

func GetNeighbors() []NeighborInfo {
	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	result := []NeighborInfo{}
	for _, neighbor := range connectedNeighbors {
		result = append(result, NeighborInfo{
			Neighbor: neighbor,
			Address:  neighbor.InitAddress.Addr + ":" + strconv.FormatInt(int64(neighbor.InitAddress.Port), 10),
			Domain: func() string {
				if neighbor.Domain == "" {
					return neighbor.InitAddress.Addr
				}
				return neighbor.Domain
			}(),
			NumberOfAllTransactions:           neighbor.Metrics.GetAllTransactionsCount(),
			NumberOfInvalidTransactions:       neighbor.Metrics.GetInvalidTransactionsCount(),
			NumberOfStaleTransactions:         neighbor.Metrics.GetStaleTransactionsCount(),
			NumberOfNewTransactions:           neighbor.Metrics.GetNewTransactionsCount(),
			NumberOfSentTransactions:          neighbor.Metrics.GetSentTransactionsCount(),
			NumberOfDroppedSentPackets:        neighbor.Metrics.GetDroppedSendPacketsCount(),
			NumberOfRandomTransactionRequests: neighbor.Metrics.GetRandomTransactionRequestsCount(),
			ConnectionType:                    "tcp",
			Connected:                         true,
		})
	}

	for _, originAddr := range reconnectPool {
		result = append(result, NeighborInfo{
			Address:        originAddr.Addr + ":" + strconv.FormatInt(int64(originAddr.Port), 10),
			Domain:         originAddr.Addr,
			ConnectionType: "tcp",
			Connected:      false,
		})
	}

	return result
}
