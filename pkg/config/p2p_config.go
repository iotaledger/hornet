package config

const (
	// Defines the bind addresses of this node.
	CfgP2PBindAddresses = "p2p.bindAddresses"
	// Defines the path to the peerstore.
	CfgP2PPeerStorePath = "p2p.peerStore.path"
	// Defines the high watermark to use within the connection manager.
	CfgP2PConnMngHighWatermark = "p2p.connectionManager.highWatermark"
	// Defines the low watermark to use within the connection manager.
	CfgP2PConnMngLowWatermark = "p2p.connectionManager.lowWatermark"
	// Defines the static peers this node should retain a connection to.
	CfgP2PPeers = "p2p.peers"
)

func init() {
	configFlagSet.StringSlice(CfgP2PBindAddresses, []string{"/ip4/127.0.0.1/tcp/15600"}, "the bind addresses for this node")
	configFlagSet.String(CfgP2PPeerStorePath, "./p2pstore", "the path to the peer store")
	configFlagSet.Int(CfgP2PConnMngHighWatermark, 10, "defines the threshold up on which connections count truncates to the lower watermark")
	configFlagSet.Int(CfgP2PConnMngLowWatermark, 5, "defines the minimum connections count to hold after the high watermark was reached")
	configFlagSet.StringSlice(CfgP2PPeers, []string{}, "the static peers this node should retain a connection to")
}
