package peering

import (
	"fmt"
	"net"
	"time"

	"github.com/iotaledger/hive.go/iputils"
	"github.com/iotaledger/hive.go/network"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol"
)

// Reconnect instructs the manager to initiate connections to all peers residing in the reconnect pool.
func (m *Manager) Reconnect() {
	m.Lock()
	peersToConnectTo := make([]*peer.Peer, 0)

	if len(m.reconnect) == 0 || m.shutdown.Load() {
		m.Unlock()
		return
	}

	m.Events.Reconnecting.Trigger(int32(len(m.reconnect)))

	// try to lookup each address and if we fail to do so, keep the address in the reconnect pool
next:
	for k, reconnectInfo := range m.reconnect {
		originAddr := reconnectInfo.OriginAddr
		peerAddrs, err := iputils.GetIPAddressesFromHost(originAddr.Addr)
		if err != nil {
			m.Events.IPLookupError.Trigger(err)
			continue
		}

		// cache ips
		reconnectInfo.mu.Lock()
		reconnectInfo.CachedIPs = peerAddrs
		reconnectInfo.mu.Unlock()

		prefIP := peerAddrs.GetPreferredAddress(originAddr.PreferIPv6)

		// don't do any new connection attempts if the peer is already connected
		ips := make([]string, 0)
		for ip := range peerAddrs.IPs {
			id := peer.NewID(ip.String(), originAddr.Port)
			if p, alreadyConnected := m.connected[id]; alreadyConnected {
				m.Events.ReconnectRemovedAlreadyConnected.Trigger(p)
				delete(m.reconnect, k)
				continue next
			}
			ips = append(ips, ip.String())
		}

		// whitelist all possible combinations for this peer ID
		m.Whitelist(ips, reconnectInfo.OriginAddr.Port)

		// create a new outbound peer and inject autopeering metadata if available
		p := peer.NewOutboundPeer(originAddr, prefIP, originAddr.Port, peerAddrs)
		if reconnectInfo.Autopeering != nil {
			p.Autopeering = reconnectInfo.Autopeering
		}
		peersToConnectTo = append(peersToConnectTo, p)
	}
	m.Unlock()

	for _, p := range peersToConnectTo {

		m.Lock()
		m.moveFromReconnectPoolToHandshaking(p)
		m.Unlock()

		if p.Autopeering != nil {
			m.Events.AutopeeredPeerHandshaking.Trigger(p)
		}

		if err := m.connect(p); err != nil {
			m.Events.Error.Trigger(err)
			m.Lock()
			m.moveFromConnectedToReconnectPool(p)
			m.Unlock()
			continue
		}

		m.SetupEventHandlers(p)

		// kicks of the protocol by sending the handshake packet and then reading inbound data
		go p.Protocol.Start()
	}
}

// adds the given peers to the reconnect pool.
func (m *Manager) moveInitialPeersToReconnectPool(peers []*config.PeerConfig) {
	for _, peerConf := range peers {
		if peerConf.ID == "" {
			continue
		}

		originAddr, err := iputils.ParseOriginAddress(peerConf.ID)
		if err != nil {
			panic(errors.Wrapf(err, "invalid peer address %s", peerConf.ID))
		}

		originAddr.PreferIPv6 = peerConf.PreferIPv6
		originAddr.Alias = peerConf.Alias

		// no need to lock the manager in the configure stage
		m.moveToReconnectPool(&reconnectinfo{OriginAddr: originAddr})
	}
}

// creates and initiates the connection to the given peer.
func (m *Manager) connect(p *peer.Peer) error {
	addr := fmt.Sprintf("%s:%d", iputils.IPToString(p.PrimaryAddress), p.InitAddress.Port)
	conn, err := net.DialTimeout("tcp", addr, time.Duration(2)*time.Second)
	if err != nil {
		return fmt.Errorf("can't connect to %s: %w", p.ID, err)
	}

	p.Conn = network.NewManagedConnection(conn)
	p.Conn.SetWriteTimeout(connectionWriteTimeout)
	p.Protocol = protocol.New(p.Conn)
	return nil
}

// removes the given peer from the reconnect pool.
func (m *Manager) removeFromReconnectPool(p *peer.Peer) {
	for key, reconnectInfo := range m.reconnect {
		reconnectInfo.mu.Lock()

		if reconnectInfo.CachedIPs == nil {
			ips, err := iputils.GetIPAddressesFromHost(reconnectInfo.OriginAddr.Addr)
			if err != nil {
				m.Events.IPLookupError.Trigger(err)
				reconnectInfo.mu.Unlock()
				continue
			}
			reconnectInfo.CachedIPs = ips
		}

		for ip := range reconnectInfo.CachedIPs.IPs {
			if p.ID == peer.NewID(ip.String(), reconnectInfo.OriginAddr.Port) {
				// if the reconnect pool's entry can't be parsed, it must be the original domain name
				if net.ParseIP(reconnectInfo.OriginAddr.Addr) == nil {
					p.InitAddress.Addr = reconnectInfo.OriginAddr.Addr
				}

				// if the reconnect pool's entry isn't empty, it must be the origial alias
				if p.InitAddress.Alias == "" && reconnectInfo.OriginAddr.Alias != "" {
					p.InitAddress.Alias = reconnectInfo.OriginAddr.Alias
				}

				// make an union of what the reconnect pool entry had
				p.Addresses = reconnectInfo.CachedIPs.Union(p.Addresses)

				// delete out of reconnect pool
				delete(m.reconnect, key)
				m.Events.PeerRemovedFromReconnectPool.Trigger(key)
				break
			}
		}
		reconnectInfo.mu.Unlock()
	}
}
