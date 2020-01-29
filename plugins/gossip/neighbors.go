package gossip

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/shutdown"
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
	reconnectPool       = make(map[string]*reconnectneighbor)
	reconnectPoolWakeup = make(chan struct{})

	// a set containing identities which are allowed to connect
	allowedIdentities = make(map[string]*peer.Peer)

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

type reconnectneighbor struct {
	mu          sync.Mutex
	OriginAddr  *iputils.OriginAddress `json:"origin_addr"`
	CachedIPs   *iputils.IPAddresses   `json:"cached_ips"`
	Autopeering *peer.Peer             `json:"peer"`
}

func availableNeighborSlotsFilled() bool {
	// while this check is not thread-safe, initiated connections will be dropped
	// when their handshaking was done but already all neighbor slots are filled
	return len(connectedNeighbors) >= parameter.NeighborsConfig.GetInt("maxNeighbors")
}

func configureNeighbors() {
	autoTetheringEnabled = parameter.NeighborsConfig.GetBool("autoTetheringEnabled")

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
		gossipLogger.Info("Closing neighbor connections ...")

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
	// The address IP address under which the neighbor is connected
	PrimaryAddress *iputils.IP
	// The IP addresses which were looked up during neighbor initialisation
	Addresses *iputils.IPAddresses
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
	// Whether the neighbor is a duplicate, as it is already connected
	Duplicate bool
	// The neighbors latest heartbeat message
	LatestHeartbeat *Heartbeat
	// Holds the peer information if this neighbor was added via autopeering
	Autopeering *peer.Peer
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
	addresses := iputils.NewIPAddresses()
	primaryAddr := &iputils.IP{IP: ip}
	addresses.Add(primaryAddr)

	// InitAddress and Identity are set in finalizeHandshake
	return &Neighbor{
		PrimaryAddress: primaryAddr,
		Addresses:      addresses,
		Events: neighborEvents{
			ProtocolConnectionEstablished: events.NewEvent(protocolCaller),
		},
		Metrics:          &neighbor.NeighborMetrics{},
		ConnectionOrigin: Inbound,
	}
}

func NewOutboundNeighbor(originAddr *iputils.OriginAddress, primaryAddr *iputils.IP, port uint16, addresses *iputils.IPAddresses) *Neighbor {
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

func addNeighborToReconnectPool(recNeigh *reconnectneighbor) {
	if _, has := reconnectPool[recNeigh.OriginAddr.String()]; has {
		return
	}
	reconnectPool[recNeigh.OriginAddr.String()] = recNeigh
	Events.NeighborPutIntoReconnectPool.Trigger(recNeigh.OriginAddr)
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
	if neighbor.Autopeering != nil {
		return
	}
	reconnectPool[neighbor.InitAddress.String()] = &reconnectneighbor{
		OriginAddr: neighbor.InitAddress,
		CachedIPs:  neighbor.Addresses,
	}
	Events.NeighborPutBackIntoReconnectPool.Trigger(neighbor)
}

func moveNeighborToConnected(neighbor *Neighbor) {
	// neighbors lock must be held by caller
	delete(inFlightNeighbors, neighbor.Identity)
	connectedNeighbors[neighbor.Identity] = neighbor

	// delete any existing neighbor from the reconnect pool
	cleanReconnectPool(neighbor)

	Events.NeighborPutIntoConnectedPool.Trigger(neighbor)
}

func moveNeighborFromConnectedToReconnectPool(neighbor *Neighbor) {
	// neighbors lock must be held by caller

	// check whether manually removed or autopeered neighbor
	if !neighbor.MoveBackToReconnectPool || neighbor.Autopeering != nil {
		return
	}

	// prevents non handshaked connections to be put back into the reconnect pool
	if _, ok := connectedNeighbors[neighbor.Identity]; !ok {
		return
	}
	delete(connectedNeighbors, neighbor.Identity)

	// remove any other reconnect pool entry where the identity would match
	cleanReconnectPool(neighbor)

	reconnectPool[neighbor.InitAddress.String()] = &reconnectneighbor{
		OriginAddr: neighbor.InitAddress,
		CachedIPs:  neighbor.Addresses,
	}
	Events.NeighborPutBackIntoReconnectPool.Trigger(neighbor)
}

func allowNeighborIdentity(neighbor *Neighbor) {
	for ip := range neighbor.Addresses.IPs {
		identity := NewNeighborIdentity(ip.String(), neighbor.InitAddress.Port)
		allowedIdentities[identity] = nil
		hostsBlacklistLock.Lock()
		delete(hostsBlacklist, ip.String())
		hostsBlacklistLock.Unlock()
	}
}

func finalizeHandshake(protocol *protocol, handshake *Handshake) error {
	// make sure only one handshake finalization process is ongoing at once
	handshakeFinalisationLock.Lock()
	defer handshakeFinalisationLock.Unlock()

	neighbor := protocol.Neighbor

	// drop the connection if in the meantime the available neighbor slots were filled
	if autoTetheringEnabled && availableNeighborSlotsFilled() {
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
		// grab autopeering information from whitelist
		neighbor.Autopeering = allowedIdentities[neighbor.Identity]
		if neighbor.Autopeering != nil {
			gossipAddr := neighbor.Autopeering.Services().Get(service.GossipKey).String()
			gossipLogger.Infof("handshaking with autopeered neighbor %s / %s", gossipAddr, neighbor.Autopeering.ID())
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
			hostsBlacklistLock.Lock()
			hostsBlacklist[neighbor.PrimaryAddress.String()] = struct{}{}
			hostsBlacklistLock.Unlock()
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
		if daemon.IsStopped() {
			return
		}
		if errors.Cause(err) == ErrNeighborAlreadyConnected {
			neighbor.Duplicate = true
		}
		gossipLogger.Warnf("protocol error on neighbor %s: %s", neighbor.IdentityOrAddress(), err.Error())
	}))

	// connection error log
	neighbor.Protocol.Conn.Events.Error.Attach(events.NewClosure(func(err error) {
		// trigger global closed event
		Events.NeighborConnectionClosed.Trigger(neighbor)
		if daemon.IsStopped() {
			return
		}
		if neighbor.Duplicate {
			return
		}
		gossipLogger.Warnf("connection error on neighbor %s: %s", neighbor.IdentityOrAddress(), err.Error())
	}))

	// automatically put the disconnected neighbor back into the reconnect pool
	// if not closed on purpose
	neighbor.Protocol.Conn.Events.Close.Attach(events.NewClosure(func() {
		if neighbor.Duplicate {
			gossipLogger.Infof("duplicate connection closed to %s", neighbor.IdentityOrAddress())
			return
		}
		gossipLogger.Infof("connection closed to %s", neighbor.IdentityOrAddress())
		if daemon.IsStopped() {
			return
		}
		neighborsLock.Lock()
		defer neighborsLock.Unlock()
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
	select {
	case reconnectPoolWakeup <- struct{}{}:
	default:
	}
}

func AddNeighbor(neighborAddr string, preferIPv6 bool, alias string, autoPeer ...*peer.Peer) error {
	originAddr, err := iputils.ParseOriginAddress(neighborAddr)
	if err != nil {
		return errors.Wrapf(err, "invalid neighbor address %s", neighborAddr)
	}

	originAddr.PreferIPv6 = preferIPv6
	originAddr.Alias = alias

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
	recNeigh := &reconnectneighbor{OriginAddr: originAddr, CachedIPs: possibleIdentities}
	if len(autoPeer) > 0 {
		recNeigh.Autopeering = autoPeer[0]
	}
	addNeighborToReconnectPool(recNeigh)

	// force reconnect attempts now
	wakeupReconnectPool()
	return nil
}

func possibleIdentitiesFromNeighborAddress(originAddr *iputils.OriginAddress) (*iputils.IPAddresses, error) {
	possibleIdentities := iputils.NewIPAddresses()
	ip := net.ParseIP(originAddr.Addr)
	if ip != nil {
		possibleIdentities.Add(&iputils.IP{IP: ip})
		return possibleIdentities, nil
	}
	ips, err := net.LookupHost(originAddr.Addr)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't lookup IPs for %s, error: %s", originAddr.Addr, err.Error())
	}
	if len(ips) == 0 {
		return nil, errors.Wrapf(ErrNoIPsFound, "no IPs found for %s", originAddr.Addr)
	}
	for _, ipAddr := range ips {
		possibleIdentities.Add(&iputils.IP{IP: net.ParseIP(ipAddr)})
	}
	return possibleIdentities, nil
}

func RemoveNeighbor(originIdentity string) error {
	originAddr, err := iputils.ParseOriginAddress(originIdentity)
	if err != nil {
		return errors.Wrapf(err, "invalid neighbor address %s", originIdentity)
	}
	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	// always remove the neighbor from the reconnect pool through its origin identity
	delete(reconnectPool, originIdentity)

	if possibleIdentities, err := possibleIdentitiesFromNeighborAddress(originAddr); err == nil {

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
			hostsBlacklistLock.Lock()
			hostsBlacklist[ip.String()] = struct{}{}
			hostsBlacklistLock.Unlock()
		}
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
			hostsBlacklistLock.Lock()
			hostsBlacklist[neigh.Identity] = struct{}{}
			hostsBlacklistLock.Unlock()
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
	DomainWithPort                    string    `json:"-"`
	Alias                             string    `json:"alias,omitempty"`
	PreferIPv6                        bool      `json:"-"`
	NumberOfAllTransactions           uint32    `json:"numberOfAllTransactions"`
	NumberOfRandomTransactionRequests uint32    `json:"numberOfRandomTransactionRequests"`
	NumberOfNewTransactions           uint32    `json:"numberOfNewTransactions"`
	NumberOfInvalidTransactions       uint32    `json:"numberOfInvalidTransactions"`
	NumberOfStaleTransactions         uint32    `json:"numberOfStaleTransactions"`
	NumberOfSentTransactions          uint32    `json:"numberOfSentTransactions"`
	NumberOfDroppedSentPackets        uint32    `json:"numberOfDroppedSentPackets"`
	ConnectionType                    string    `json:"connectionType"`
	Connected                         bool      `json:"connected"`
	AutopeeringID                     string    `json:"autopeering_id"`
}

type AutopeeringInfo struct {
	ID string `json:"string"`
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
		info := NeighborInfo{
			Neighbor:                          neighbor,
			Address:                           neighbor.Identity,
			Domain:                            neighbor.InitAddress.Addr,
			DomainWithPort:                    neighbor.InitAddress.String(),
			Alias:                             neighbor.InitAddress.Alias,
			NumberOfAllTransactions:           neighbor.Metrics.GetAllTransactionsCount(),
			NumberOfInvalidTransactions:       neighbor.Metrics.GetInvalidTransactionsCount(),
			NumberOfStaleTransactions:         neighbor.Metrics.GetStaleTransactionsCount(),
			NumberOfNewTransactions:           neighbor.Metrics.GetNewTransactionsCount(),
			NumberOfSentTransactions:          neighbor.Metrics.GetSentTransactionsCount(),
			NumberOfDroppedSentPackets:        neighbor.Metrics.GetDroppedSendPacketsCount(),
			NumberOfRandomTransactionRequests: neighbor.Metrics.GetRandomTransactionRequestsCount(),
			ConnectionType:                    "tcp",
			Connected:                         true,
			PreferIPv6:                        neighbor.InitAddress.PreferIPv6,
		}
		if neighbor.Autopeering != nil {
			info.AutopeeringID = neighbor.Autopeering.ID().String()
		}
		result = append(result, info)
	}

	for _, recNeigh := range reconnectPool {
		originAddr := recNeigh.OriginAddr
		info := NeighborInfo{
			Address:        originAddr.Addr + ":" + strconv.FormatInt(int64(originAddr.Port), 10),
			Domain:         originAddr.Addr,
			DomainWithPort: originAddr.Addr + ":" + strconv.FormatInt(int64(originAddr.Port), 10),
			Alias:          originAddr.Alias,
			ConnectionType: "tcp",
			Connected:      false,
			PreferIPv6:     originAddr.PreferIPv6,
		}
		if recNeigh.Autopeering != nil {
			info.AutopeeringID = recNeigh.Autopeering.ID().String()
		}
		result = append(result, )
	}

	return result
}

func GetNeighborsCount() int {
	neighborsLock.Lock()
	defer neighborsLock.Unlock()

	return len(connectedNeighbors) + len(reconnectPool)
}
