package p2p

import (
	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// ConfigManager handles the list of peers that are stored in the peering config.
// It calls a function if the list changed.
type ConfigManager struct {
	storeCallback func([]*PeerConfig) error
	storeOnChange bool
	peers         []*PeerConfig
}

// NewConfigManager creates a new config manager.
func NewConfigManager(storeCallback func([]*PeerConfig) error) *ConfigManager {
	return &ConfigManager{
		storeCallback: storeCallback,
		storeOnChange: false,
		peers:         []*PeerConfig{},
	}
}

// GetPeers returns all known peers.
func (pm *ConfigManager) GetPeers() []*PeerConfig {
	return pm.peers
}

// AddPeer adds a peer to the config manager.
func (pm *ConfigManager) AddPeer(multiAddress multiaddr.Multiaddr, alias string) error {

	newPeerAddrInfo, err := peer.AddrInfoFromP2pAddr(multiAddress)
	if err != nil {
		return err
	}

	for _, p := range pm.peers {
		multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
		if err != nil {
			// ignore wrong values in the config file
			continue
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			// ignore wrong values in the config file
			continue
		}

		if addrInfo.ID == newPeerAddrInfo.ID {
			return errors.New("peer already exists")
		}
	}

	// no peer with the same ID found, add the new one
	pm.peers = append(pm.peers, &PeerConfig{
		MultiAddress: multiAddress.String(),
		Alias:        alias,
	})

	return pm.store()
}

// RemovePeer removes a peer from the config manager.
func (pm *ConfigManager) RemovePeer(peerID peer.ID) error {

	for i, p := range pm.peers {
		multiAddr, err := multiaddr.NewMultiaddr(p.MultiAddress)
		if err != nil {
			// ignore wrong values in the config file
			continue
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			// ignore wrong values in the config file
			continue
		}

		if addrInfo.ID == peerID {
			// delete without preserving order
			pm.peers[i] = pm.peers[len(pm.peers)-1]
			pm.peers[len(pm.peers)-1] = nil // avoid potential memory leak
			pm.peers = pm.peers[:len(pm.peers)-1]
			return pm.store()
		}
	}

	return errors.New("peer not found")
}

// StoreOnChange sets whether storing changes to the config is active or not.
func (pm *ConfigManager) StoreOnChange(store bool) {
	pm.storeOnChange = store
}

// store calls the storeCallback if storeOnChange is active.
func (pm *ConfigManager) store() error {
	if !pm.storeOnChange {
		return nil
	}

	return pm.storeCallback(pm.peers)
}
