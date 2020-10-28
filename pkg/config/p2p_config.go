package config

const (
	// Defines the private key used to derive the node identity (optional).
	CfgP2PIdentityPrivKey = "p2p.identityPrivateKey"
	// Defines the bind addresses of this node.
	CfgP2PBindMultiAddresses = "p2p.bindMultiAddresses"
	// Defines the path to the peerstore.
	CfgP2PPeerStorePath = "p2p.peerStore.path"
	// Defines the high watermark to use within the connection manager.
	CfgP2PConnMngHighWatermark = "p2p.connectionManager.highWatermark"
	// Defines the low watermark to use within the connection manager.
	CfgP2PConnMngLowWatermark = "p2p.connectionManager.lowWatermark"
	// Defines the static peers this node should retain a connection to.
	CfgP2PPeers = "p2p.peers"
	// Defines the aliases of the static peers (must be the same length like CfgP2PPeers).
	CfgP2PPeerAliases = "p2p.peerAliases"
	// Defines the number of seconds to wait before trying to reconnect to a disconnected peer.
	CfgP2PReconnectIntervalSeconds = "p2p.reconnectIntervalSeconds"
)

func init() {
	configFlagSet.String(CfgP2PIdentityPrivKey, "", "private key used to derive the node identity (optional)")
	configFlagSet.StringSlice(CfgP2PBindMultiAddresses, []string{"/ip4/127.0.0.1/tcp/15600"}, "the bind addresses for this node")
	configFlagSet.String(CfgP2PPeerStorePath, "./p2pstore", "the path to the peer store")
	configFlagSet.Int(CfgP2PConnMngHighWatermark, 10, "defines the threshold up on which connections count truncates to the lower watermark")
	configFlagSet.Int(CfgP2PConnMngLowWatermark, 5, "defines the minimum connections count to hold after the high watermark was reached")
	peeringFlagSet.StringSlice(CfgP2PPeers, []string{}, "the static peers this node should retain a connection to")
	peeringFlagSet.StringSlice(CfgP2PPeerAliases, []string{}, "the aliases of the static peers (must be the same amount like \"p2p.peers\")")
	configFlagSet.Int(CfgP2PReconnectIntervalSeconds, 30, "the number of seconds to wait before trying to reconnect to a disconnected peer")
}
