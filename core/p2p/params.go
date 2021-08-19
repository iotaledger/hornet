package p2p

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// Defines the bind addresses of this node.
	CfgP2PBindMultiAddresses = "p2p.bindMultiAddresses"
	// Defines the high watermark to use within the connection manager.
	CfgP2PConnMngHighWatermark = "p2p.connectionManager.highWatermark"
	// Defines the low watermark to use within the connection manager.
	CfgP2PConnMngLowWatermark = "p2p.connectionManager.lowWatermark"
	// Defines the private key used to derive the node identity (optional).
	CfgP2PIdentityPrivKey = "p2p.identityPrivateKey"
	// Defines the path to the p2p database.
	CfgP2PDatabasePath = "p2p.db.path"
	// Defines the time to wait before trying to reconnect to a disconnected peer.
	CfgP2PReconnectInterval = "p2p.reconnectInterval"
	// Defines the static peers this node should retain a connection to (config file).
	CfgP2PPeers = "p2p.peers"
	// Defines the aliases of the static peers (must be the same length like CfgP2PPeers) (CLI).
	CfgP2PPeerAliases = "p2p.peerAliases"
	// Defines the static peers this node should retain a connection to (CLI).
	CfgPeers = "peers"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.StringSlice(CfgP2PBindMultiAddresses, []string{"/ip4/127.0.0.1/tcp/15600"}, "the bind addresses for this node")
			fs.Int(CfgP2PConnMngHighWatermark, 10, "the threshold up on which connections count truncates to the lower watermark")
			fs.Int(CfgP2PConnMngLowWatermark, 5, "the minimum connections count to hold after the high watermark was reached")
			fs.String(CfgP2PIdentityPrivKey, "", "private key used to derive the node identity (optional)")
			fs.String(CfgP2PDatabasePath, "p2pstore", "the path to the p2p database")
			fs.Duration(CfgP2PReconnectInterval, 30*time.Second, "the time to wait before trying to reconnect to a disconnected peer")
			return fs
		}(),
		"peeringConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.StringSlice(CfgP2PPeers, []string{}, "the static peers this node should retain a connection to (CLI)")
			fs.StringSlice(CfgP2PPeerAliases, []string{}, "the aliases of the static peers (must be the same amount like \"p2p.peers\") (CLI)")
			return fs
		}(),
	},
	Masked: []string{CfgP2PIdentityPrivKey},
}
