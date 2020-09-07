package peering

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	autopeering "github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/network"
	"github.com/iotaledger/hive.go/network/tcp"
	"github.com/labstack/gommon/log"
	"go.uber.org/atomic"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol"
	"github.com/gohornet/hornet/pkg/protocol/handshake"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

const (
	isNeighborSyncedThreshold        = 2
	updateNeighborsCountCooldownTime = time.Duration(2 * time.Second)
	connectionWriteTimeout           = 5 * time.Second
)

var (
	// ErrPeeringSlotsFilled is returned when all available peering slots are filled.
	ErrPeeringSlotsFilled = errors.New("peering slots filled")
	// ErrNonMatchingMWM is returned when the MWM doesn't match this node's MWM.
	ErrNonMatchingMWM = errors.New("used MWM doesn't match")
	// ErrNonMatchingCooAddr is returned when the Coo address doesn't match this node's Coo address.
	ErrNonMatchingCooAddr = errors.New("used coo addr doesn't match")
	// ErrNonMatchingSrvSocketPort is returned when the server socket port doesn't match.
	ErrNonMatchingSrvSocketPort = errors.New("advertised server socket port doesn't match")
	// ErrUnknownPeerID is returned when an unknown peer tried to connect.
	ErrUnknownPeerID = errors.New("peer ID is not known")
	// ErrPeerAlreadyConnected is returned when a given peer is already connected.
	ErrPeerAlreadyConnected = errors.New("peer is already connected")
	// ErrPeerAlreadyInReconnectPool is returned when a given peer is already in the reconnect pool.
	ErrPeerAlreadyInReconnect = errors.New("peer is already in the reconnect pool")
	// ErrManagerIsShutdown is returned when the manager is shutdown.
	ErrManagerIsShutdown = errors.New("peering manager is shutdown")
)

// NewManager creates a new manager instance with the given Options and moves the given peers
// into the reconnect pool.
func NewManager(opts Options, peers ...*config.PeerConfig) *Manager {
	m := &Manager{
		Events: Events{
			ConnectedAutopeeredPeer:               events.NewEvent(peer.Caller),
			PeerHandshakingIncoming:               events.NewEvent(events.StringCaller),
			PeerConnected:                         events.NewEvent(peer.Caller),
			PeerDisconnected:                      events.NewEvent(peer.Caller),
			PeerRemovedFromReconnectPool:          events.NewEvent(peer.Caller),
			PeerMovedIntoReconnectPool:            events.NewEvent(peer.OriginAddressCaller),
			PeerMovedFromConnectedToReconnectPool: events.NewEvent(peer.Caller),
			PeerHandshakingOutgoing:               events.NewEvent(peer.Caller),
			Reconnecting:                          events.NewEvent(events.Int32Caller),
			ReconnectRemovedAlreadyConnected:      events.NewEvent(peer.Caller),
			AutopeeredPeerHandshaking:             events.NewEvent(peer.Caller),
			AutopeeredPeerBecameStatic:            events.NewEvent(peer.IdentityCaller),
			IPLookupError:                         events.NewEvent(events.ErrorCaller),
			Shutdown:                              events.NewEvent(events.CallbackCaller),
			Error:                                 events.NewEvent(events.ErrorCaller),
		},
		tcpServer: tcp.NewServer(),
		connected: map[string]*peer.Peer{},
		reconnect: map[string]*reconnectinfo{},
		whitelist: map[string]*autopeering.Peer{},
		blacklist: map[string]struct{}{},
		Opts:      opts,
	}
	m.moveInitialPeersToReconnectPool(peers)
	return m
}

// Manager manages a set of connected peers and those to which the node
// wants to create a connection to.
type Manager struct {
	sync.RWMutex

	shutdown atomic.Bool

	// Peering related events.
	Events Events
	// manager options.
	Opts Options

	// the TCP server instance used to handle incoming connections.
	tcpServer *tcp.TCPServer
	// holds currently connected peers.
	connected map[string]*peer.Peer
	// holds peers to which we want to connect to.
	reconnect map[string]*reconnectinfo
	// defines the set of allowed peer identities.
	whitelist   map[string]*autopeering.Peer
	whitelistMu sync.Mutex
	// defines a set of blacklisted IP addresses.
	blacklist   map[string]struct{}
	blacklistMu sync.Mutex
	// used to enforce one handshake verification at a time.
	handshakeVerifyMu sync.Mutex

	// only used by ConnectedAndSyncedPeerCount
	connectedNeighborsCount  uint8
	syncedNeighborsCount     uint8
	neighborsCountLastUpdate time.Time
}

type reconnectinfo struct {
	mu          sync.Mutex
	OriginAddr  *iputils.OriginAddress `json:"origin_addr"`
	CachedIPs   *iputils.IPAddresses   `json:"cached_ips"`
	Autopeering *autopeering.Peer      `json:"peer"`
}

// Options defines options for the Manager.
type Options struct {
	ValidHandshake handshake.Handshake
	// The max amount of connected peers (non-autopeering).
	MaxConnected int
	// Whether to allow connections from any peer.
	AcceptAnyPeer bool
	// Inbound connection bind address.
	BindAddress string
}

// Events defines events fired regarding peering.
type Events struct {
	// Fired when an autopeered peer was connected and handshaked.
	ConnectedAutopeeredPeer *events.Event
	// Fired when an IP lookup error occurs.
	IPLookupError *events.Event
	// Fired when the handshaking phase with a peer was successfully executed.
	PeerConnected *events.Event
	// Fired when a peer is disconnected and completely removed from the Manager.
	PeerDisconnected *events.Event
	// Fired when a peer is removed from the reconnect pool.
	PeerRemovedFromReconnectPool *events.Event
	// Fired when a peer is moved into the reconnect pool.
	PeerMovedIntoReconnectPool *events.Event
	// Fired when a peer is moved from connected to the reconnect pool.
	PeerMovedFromConnectedToReconnectPool *events.Event
	// Fired when the handshaking phase of an outgoing peer connection is initiated.
	PeerHandshakingOutgoing *events.Event
	// Fired when the handshaking phase of an incoming peer connection is initiated.
	PeerHandshakingIncoming *events.Event
	// Fired when the handshaking phase with an outbound autopeered peer is initiated.
	AutopeeredPeerHandshaking *events.Event
	// Fired when an autopeered peer was added as a static neighbor.
	AutopeeredPeerBecameStatic *events.Event
	// Fired when a reconnect is initiated over the entire reconnect pool.
	Reconnecting *events.Event
	// Fired when during the reconnect phase a peer is already connected.
	ReconnectRemovedAlreadyConnected *events.Event
	// Fired when the manager has been successfully shutdown.
	Shutdown *events.Event
	// Fired when internal errors occur.
	Error *events.Event
}

// IsStaticallyPeered tells if the peer is already statically peered.
// all possible IDs for the given IP addresses/port combination are checked.
func (m *Manager) IsStaticallyPeered(ips []string, port uint16) bool {
	m.RLock()
	defer m.RUnlock()

	for _, peerIP := range ips {
		peerID := peer.NewID(peerIP, port)

		// check all connected peers
		for _, p := range m.connected {
			for connectedIP := range p.Addresses.IPs {
				connectedID := peer.NewID(connectedIP.String(), p.InitAddress.Port)

				if connectedID == peerID {
					return true
				}
			}
		}

		// check all peers in the reconnect pool
		for _, reconnectInfo := range m.reconnect {
			// if the static peer has no DNS records, CachedIPs would be nil
			if reconnectInfo.CachedIPs == nil {
				continue
			}
			for reconnectInfoIP := range reconnectInfo.CachedIPs.IPs {
				reconnectID := peer.NewID(reconnectInfoIP.String(), reconnectInfo.OriginAddr.Port)

				if reconnectID == peerID {
					return true
				}
			}
		}
	}

	return false
}

// Blacklisted tells whether the given IP address is blacklisted.
func (m *Manager) Blacklisted(ip string) bool {
	m.blacklistMu.Lock()
	_, blacklisted := m.blacklist[ip]
	m.blacklistMu.Unlock()
	return blacklisted
}

// Blacklist blacklists the given IP from connecting.
func (m *Manager) Blacklist(ip string) {
	m.blacklistMu.Lock()
	m.blacklist[ip] = struct{}{}
	m.blacklistMu.Unlock()
}

// BlacklistRemove removes the blacklist entry for the given IP address.
func (m *Manager) BlacklistRemove(ip string) {
	m.blacklistMu.Lock()
	delete(m.blacklist, ip)
	m.blacklistMu.Unlock()
}

// Whitelisted tells whether the given ID is whitelisted.
func (m *Manager) Whitelisted(id string) (*autopeering.Peer, bool) {
	m.whitelistMu.Lock()
	autopeeringPeer, whitelisted := m.whitelist[id]
	m.whitelistMu.Unlock()
	return autopeeringPeer, whitelisted
}

// Whitelist whitelists all possible IDs for the given IP addresses/port combination (also removes any excess blacklist entry).
// Optionally takes in autopeering metadata which is passed further to the peer once its connected.
func (m *Manager) Whitelist(ips []string, port uint16, autopeeringPeer ...*autopeering.Peer) {
	m.whitelistMu.Lock()
	defer m.whitelistMu.Unlock()
	for _, ip := range ips {
		id := peer.NewID(ip, port)
		if len(autopeeringPeer) > 0 {
			m.whitelist[id] = autopeeringPeer[0]
		} else {
			m.whitelist[id] = nil
		}
		m.BlacklistRemove(ip)
	}
}

// WhitelistRemove removes the whitelist entry for the given IP address.
func (m *Manager) WhitelistRemove(id string) {
	m.whitelistMu.Lock()
	delete(m.whitelist, id)
	m.whitelistMu.Unlock()
}

// PeerConsumerFunc is a function which consumes a peer.
// If it returns false, it signals that no further calls should be made to the function.
type PeerConsumerFunc func(p *peer.Peer) bool

// ForAllConnected executes the given function for each currently connected peer until
// abort is returned from within the consumer function. The consumer function is
// only called on peers who are handshaked.
func (m *Manager) ForAllConnected(f PeerConsumerFunc) {
	m.RLock()
	defer m.RUnlock()
	for _, p := range m.connected {
		if !p.Handshaked() {
			continue
		}
		if !f(p) {
			break
		}
	}
}

// ForAll executes the given function for each peer until
// abort is returned from within the consumer function.
func (m *Manager) ForAll(f PeerConsumerFunc) {
	m.RLock()
	defer m.RUnlock()
	for _, p := range m.connected {
		if !f(p) {
			return
		}
	}
	for _, p := range m.reconnect {
		peer := &peer.Peer{
			ID:          p.OriginAddr.Addr,
			InitAddress: p.OriginAddr,
			Addresses:   p.CachedIPs,
			Autopeering: p.Autopeering,
		}
		if !f(peer) {
			return
		}
	}
}

// AnySTINGPeerConnected returns true if any of the connected, handshaked peers supports the STING protocol.
func (m *Manager) AnySTINGPeerConnected() bool {
	stingPeerConnected := false

	m.ForAllConnected(func(p *peer.Peer) bool {
		if !p.Protocol.Supports(sting.FeatureSet) {
			return true
		}

		stingPeerConnected = true
		return false
	})

	return stingPeerConnected
}

// PeerInfos returns snapshots of the currently connected and in the reconnect pool residing peers.
func (m *Manager) PeerInfos() []*peer.Info {
	m.RLock()
	defer m.RUnlock()
	infos := make([]*peer.Info, 0)
	for _, p := range m.connected {
		info := p.Info()
		info.Connected = true
		infos = append(infos, info)
	}
	for _, reconnectInfo := range m.reconnect {
		originAddr := reconnectInfo.OriginAddr
		addrStr := fmt.Sprintf("%s:%d", originAddr.Addr, originAddr.Port)
		info := &peer.Info{
			Address:        addrStr,
			Domain:         originAddr.Addr,
			DomainWithPort: addrStr,
			Alias:          originAddr.Alias,
			ConnectionType: "tcp",
			Connected:      false,
			Autopeered:     false,
			PreferIPv6:     originAddr.PreferIPv6,
		}
		if reconnectInfo.Autopeering != nil {
			info.Autopeered = true
			info.AutopeeringID = reconnectInfo.Autopeering.ID().String()
		}
		infos = append(infos, info)
	}
	return infos
}

// PeerCount returns the current count of connected and in the reconnect pool residing peers.
func (m *Manager) PeerCount() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.connected) + len(m.reconnect)
}

// ConnectedPeerCount returns the current count of connected peers.
func (m *Manager) ConnectedPeerCount() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.connected)
}

// ConnectedPeerCount returns the current count of connected peers.
// it has a cooldown time to not update too frequently.
func (m *Manager) ConnectedAndSyncedPeerCount() (uint8, uint8) {
	m.RLock()
	defer m.RUnlock()

	// check cooldown time
	if time.Since(m.neighborsCountLastUpdate) < updateNeighborsCountCooldownTime {
		return m.connectedNeighborsCount, m.syncedNeighborsCount
	}

	lsi := tangle.GetLatestMilestoneIndex()

	m.connectedNeighborsCount = 0
	m.syncedNeighborsCount = 0
	for _, p := range m.connected {
		if m.connectedNeighborsCount < 255 {
			// do no count more than 255 neighbors
			m.connectedNeighborsCount++
		}

		if p.LatestHeartbeat == nil {
			continue
		}

		latestIndex := p.LatestHeartbeat.LatestMilestoneIndex
		if latestIndex < lsi {
			latestIndex = lsi
		}

		if p.LatestHeartbeat.SolidMilestoneIndex < (latestIndex - isNeighborSyncedThreshold) {
			// node not synced
			continue
		}

		m.syncedNeighborsCount++
		if m.syncedNeighborsCount == 255 {
			// do no count more than 255 synced neighbors
			break
		}
	}
	m.neighborsCountLastUpdate = time.Now()

	return m.connectedNeighborsCount, m.syncedNeighborsCount
}

// SlotsFilled checks whether all available peer slots are filled.
func (m *Manager) SlotsFilled() bool {
	staticCount := 0

	m.ForAllConnected(func(p *peer.Peer) bool {
		if p.Autopeering != nil {
			return true
		}

		staticCount++
		return true
	})

	return staticCount >= m.Opts.MaxConnected
}

// SetupEventHandlers inits the event handlers for handshaking, the underlying connection and errors.
func (m *Manager) SetupEventHandlers(p *peer.Peer) {

	onProtocolReceive := events.NewClosure(p.Protocol.Receive)

	onConnectionError := events.NewClosure(func(err error) {
		if p.Disconnected {
			return
		}
		m.Events.Error.Trigger(err)
		if closeErr := p.Conn.Close(); closeErr != nil {
			m.Events.Error.Trigger(closeErr)
		}
	})

	onProtocolError := events.NewClosure(func(err error) {
		if p.Disconnected {
			return
		}
		m.Events.Error.Trigger(err)
		if closeErr := p.Conn.Close(); closeErr != nil {
			m.Events.Error.Trigger(closeErr)
		}
	})

	onConnectionClose := events.NewClosure(func() {
		m.Lock()
		m.moveFromConnectedToReconnectPool(p)
		m.Unlock()

		p.Conn.Events.ReceiveData.Detach(onProtocolReceive)
		p.Conn.Events.Error.Detach(onConnectionError)
		p.Protocol.Events.Error.Detach(onProtocolError)
	})

	// pipe data from the connection into the protocol
	p.Conn.Events.ReceiveData.Attach(onProtocolReceive)

	// propagate errors up
	p.Conn.Events.Error.Attach(onConnectionError)

	// any error on the protocol level resolves to shutting down the connection
	p.Protocol.Events.Error.Attach(onProtocolError)

	// move peer to reconnected pool and detach all events
	p.Conn.Events.Close.Attach(onConnectionClose)

	m.setupHandshakeEventHandlers(p)
}

// Add adds a new peer to the reconnect pool and immediately invokes a connection attempt.
// The peer is not added if it is already connected or the given address is invalid.
func (m *Manager) Add(addr string, preferIPv6 bool, alias string, autoPeer ...*autopeering.Peer) error {

	originAddr, err := iputils.ParseOriginAddress(addr)
	if err != nil {
		return fmt.Errorf("invalid peer address '%s': %w", addr, err)
	}

	originAddr.PreferIPv6 = preferIPv6
	originAddr.Alias = alias

	// check whether the peer is already connected by examining all IP addresses
	possibleIPs, err := iputils.GetIPAddressesFromHost(originAddr.Addr)
	if err != nil {
		return err
	}

	m.Lock()

	reconnect := false
	isAutopeer := len(autoPeer) > 0

	defer func() {
		m.Unlock()
		if reconnect {
			m.Reconnect()
		}
	}()

	// check whether the peer is already connected or in the reconnect pool
	// given any of the IP addresses to which the peer address resolved to
	for ip := range possibleIPs.IPs {
		// check whether already in connected pool
		id := peer.NewID(ip.String(), originAddr.Port)
		if peer, exists := m.connected[id]; exists {
			if !isAutopeer && peer.Autopeering != nil {
				autopeeringIdentity := peer.Autopeering.Identity

				// mark the autopeer as statically connected now
				peer.Autopeering = nil

				// Remove the autopeering entry in the Selector (this will not drop the connection because we set "Autopeering" to nil)
				m.Events.AutopeeredPeerBecameStatic.Trigger(autopeeringIdentity)

				// no need to drop the connection
				return nil
			}
			return fmt.Errorf("%w: '%s' is already connected as '%s'", ErrPeerAlreadyConnected, originAddr.String(), id)
		}

		// check whether already in reconnect pool
		if reconnectInfo, exists := m.reconnect[originAddr.String()]; exists {
			if !isAutopeer && reconnectInfo.Autopeering != nil {
				autopeeringIdentity := reconnectInfo.Autopeering.Identity

				// mark the autopeer as statically connected now
				reconnectInfo.Autopeering = nil

				// Remove the autopeering entry in the Selector (this will not drop the connection because we set "Autopeering" to nil)
				m.Events.AutopeeredPeerBecameStatic.Trigger(autopeeringIdentity)

				// force reconnect attempts now
				reconnect = true
				return nil
			}
			return fmt.Errorf("%w: '%s' is already in the reconnect pool", ErrPeerAlreadyInReconnect, originAddr.String())
		}
	}

	// construct reconnect info
	reconnectInfo := &reconnectinfo{OriginAddr: originAddr, CachedIPs: possibleIPs}
	if isAutopeer {
		reconnectInfo.Autopeering = autoPeer[0]
	}

	m.moveToReconnectPool(reconnectInfo)

	// force reconnect attempts now
	reconnect = true
	return nil
}

// Remove tries to remove and close any open connections for peers which are identifiable through the given ID.
func (m *Manager) Remove(id string) error {
	originAddr, err := iputils.ParseOriginAddress(id)
	if err != nil {
		return fmt.Errorf("%w: invalid peer address '%s'", err, id)
	}
	m.Lock()
	defer m.Unlock()

	// make sure the peer is removed by all its possible IDs by going
	// through each resolved IP address from the lookup
	delete(m.reconnect, id)
	if possibleIPs, err := iputils.GetIPAddressesFromHost(originAddr.Addr); err == nil {
		for ip := range possibleIPs.IPs {
			otherID := peer.NewID(ip.String(), originAddr.Port)

			// close the connection of the peer and remove it from the connected pool
			if p, exists := m.connected[otherID]; exists {
				p.MoveBackToReconnectPool = false
				delete(m.connected, otherID)
				p.Disconnected = true
				if p.Protocol != nil && p.Conn != nil {
					_ = p.Conn.Close()
				}
				m.Events.PeerDisconnected.Trigger(p)
			}

			// remove entries in the whitelist and reconnect pool
			// and ensure that the ID is subsequently blacklisted
			delete(m.reconnect, otherID)
			m.WhitelistRemove(otherID)
			m.Blacklist(ip.String())
		}
	}

	// also remove the peer if the origin address matches:
	// i.e node.example.com == node.example.com
	// this could happen if the DNS entry for the given peer updated and hence
	// just matching by IP/Port wouldn't render any peer to be removed
	for _, p := range m.connected {
		if id != p.InitAddress.String() {
			continue
		}
		p.Disconnected = true
		p.MoveBackToReconnectPool = false
		delete(m.connected, id)
		_ = p.Conn.Close()
		m.Events.PeerDisconnected.Trigger(p)
		delete(m.reconnect, p.ID)
		m.WhitelistRemove(p.ID)
		m.Blacklist(p.PrimaryAddress.String())
	}

	return nil
}

// Listen starts the peering server to listen for incoming connections.
func (m *Manager) Listen() error {

	// unfortunately we need to split the bind address as the TCP server API doesn't just use
	// a connection string
	addr, portStr, err := net.SplitHostPort(m.Opts.BindAddress)
	if err != nil {
		return fmt.Errorf("%w: '%s' is an invalid bind address", err, m.Opts.BindAddress)
	}

	// We assume that addr is a literal IPv6 address if it has colons.
	if strings.Contains(addr, ":") {
		addr = "[" + addr + "]"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("%w: '%s' contains an invalid port", err, m.Opts.BindAddress)
	}

	m.tcpServer.Events.Connect.Attach(events.NewClosure(func(conn *network.ManagedConnection) {
		tcpConn := conn.RemoteAddr().(*net.TCPAddr)
		if m.Blacklisted(tcpConn.IP.String()) {
			if err := conn.Close(); err != nil {
				log.Error(err)
			}
			return
		}

		m.Events.PeerHandshakingIncoming.Trigger(conn.RemoteAddr().String())

		// init peer
		p := peer.NewInboundPeer(conn.Conn.RemoteAddr())
		p.Conn = conn
		p.Conn.SetWriteTimeout(connectionWriteTimeout)
		p.Protocol = protocol.New(conn)
		m.SetupEventHandlers(p)

		// kick off protocol
		go p.Protocol.Start()
	}))

	m.tcpServer.Events.Error.Attach(events.NewClosure(func(err error) {
		m.Events.Error.Trigger(err)
	}))

	m.tcpServer.Listen(addr, port)
	return nil
}

// Shutdown shuts down the peering server and disconnect all connected peers.
func (m *Manager) Shutdown() {
	m.Lock()
	defer m.Unlock()
	m.shutdown.Store(true)

	// stop listening for incoming connections
	m.tcpServer.Shutdown()

	// clear reconnect entries
	for k := range m.reconnect {
		delete(m.reconnect, k)
	}

	// close connections
	for k, p := range m.connected {
		p.MoveBackToReconnectPool = false
		p.Disconnected = true
		// we don't care about errors while shutting down
		_ = p.Conn.Close()
		m.Events.PeerDisconnected.Trigger(p)
		delete(m.connected, k)
	}

	m.Events.Shutdown.Trigger()
}

// moves the given peer into the connected pool and removes any pending reconnects for it.
func (m *Manager) moveToConnected(p *peer.Peer) {
	m.connected[p.ID] = p
	m.removeFromReconnectPool(p)
}

// removes the given peer from the reconnect pool and adds it to the connected pool.
// also fires a PeerHandshaking event.
func (m *Manager) moveFromReconnectPoolToHandshaking(p *peer.Peer) {
	m.moveToConnected(p)
	m.Events.PeerHandshakingOutgoing.Trigger(p)
}

// moves the given peer from connected to the reconnect pool.
// and deletes any excess pending reconnects.
func (m *Manager) moveFromConnectedToReconnectPool(p *peer.Peer) {
	if _, ok := m.connected[p.ID]; !ok {
		return
	}
	delete(m.connected, p.ID)

	// prevent non handshaked, manually removed or autopeering peers to be put back into the reconnect pool
	if !p.MoveBackToReconnectPool || p.Autopeering != nil {
		return
	}

	// remove any other excess reconnect entry
	m.removeFromReconnectPool(p)

	m.reconnect[p.InitAddress.String()] = &reconnectinfo{OriginAddr: p.InitAddress, CachedIPs: p.Addresses}
	m.Events.PeerMovedFromConnectedToReconnectPool.Trigger(p)
}

// moves the given peer into the reconnect pool
func (m *Manager) moveToReconnectPool(reconnectInfo *reconnectinfo) {
	if _, has := m.reconnect[reconnectInfo.OriginAddr.String()]; has {
		return
	}
	m.reconnect[reconnectInfo.OriginAddr.String()] = reconnectInfo
	m.Events.PeerMovedIntoReconnectPool.Trigger(reconnectInfo.OriginAddr)
}
