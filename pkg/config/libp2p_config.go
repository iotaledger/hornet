package config

const (
	// Defines the bind addresses of this node.
	CfgLibp2pBindAddresses = "libp2p.bindAddresses"
	// Defines the path to the peerstore.
	CfgLibp2pPeerStorePath = "libp2p.peerStorePath"
)

func init() {
	configFlagSet.StringArray(CfgLibp2pBindAddresses, []string{"/ip4/0.0.0.0/tcp/15600"}, "the bind addresses for this node")
	configFlagSet.String(CfgLibp2pPeerStorePath, "./peerstore", "the path to the peer store")
}
