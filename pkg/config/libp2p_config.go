package config

const (
	// Defines the bind addresses of this node.
	CfgLibp2pBindAddresses = "libp2p.bindAddresses"
	// Defines the path to the peerstore.
	CfgLibp2pPeerStorePath = "libp2p.peerStore.path"
	// Defines the high watermark to use within the connection manager.
	CfgLibp2pConnMngHighWatermark = "libp2p.connectionManager.highWatermark"
	// Defines the low watermark to use within the connection manager.
	CfgLibp2pConnMngLowWatermark = "libp2p.connectionManager.lowWatermark"
)

func init() {
	configFlagSet.StringArray(CfgLibp2pBindAddresses, []string{"/ip4/0.0.0.0/tcp/15600"}, "the bind addresses for this node")
	configFlagSet.String(CfgLibp2pPeerStorePath, "./peerstore", "the path to the peer store")
	configFlagSet.Int(CfgLibp2pConnMngHighWatermark, 10, "defines the threshold up on which connections count truncates to the lower watermark")
	configFlagSet.Int(CfgLibp2pConnMngLowWatermark, 5, "defines the minimum connections count to hold after the high watermark was reached")
}
