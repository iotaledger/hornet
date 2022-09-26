package p2p

import (
	"time"

	"github.com/iotaledger/hive.go/core/app"
)

const (
	// CfgPeers defines the static peers this node should retain a connection to (CLI).
	CfgPeers = "peers"
)

// ParametersP2P contains the definition of the parameters used by p2p.
type ParametersP2P struct {
	// Defines the bind addresses of this node.
	BindMultiAddresses []string `default:"/ip4/0.0.0.0/tcp/15600,/ip6/::/tcp/15600" usage:"the bind addresses for this node"`

	ConnectionManager struct {
		// Defines the high watermark to use within the connection manager.
		HighWatermark int `default:"10" usage:"the threshold up on which connections count truncates to the lower watermark"`
		// Defines the low watermark to use within the connection manager.
		LowWatermark int `default:"5" usage:"the minimum connections count to hold after the high watermark was reached"`
	}

	// Defines the private key used to derive the node identity (optional).
	IdentityPrivateKey string `default:"" usage:"private key used to derive the node identity (optional)"`

	Database struct {
		// Defines the path to the p2p database.
		Path string `default:"shimmer/p2pstore" usage:"the path to the p2p database"`
	} `name:"db"`

	// Defines the time to wait before trying to reconnect to a disconnected peer.
	ReconnectInterval time.Duration `default:"30s" usage:"the time to wait before trying to reconnect to a disconnected peer"`
}

// ParametersPeers contains the definition of the parameters used by peers.
type ParametersPeers struct {
	// Defines the static peers this node should retain a connection to (CLI).
	Peers []string `default:"" usage:"the static peers this node should retain a connection to (CLI)"`
	// Defines the aliases of the static peers (must be the same length like CfgP2PPeers) (CLI).
	PeerAliases []string `default:"" usage:"the aliases of the static peers (must be the same amount like \"p2p.peers\""`
}

var ParamsP2P = &ParametersP2P{}
var ParamsPeers = &ParametersPeers{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"p2p": ParamsP2P,
	},
	AdditionalParams: map[string]map[string]any{
		"peeringConfig": {
			"p2p": ParamsPeers,
		},
	},
	Masked: []string{"p2p.identityPrivateKey"},
}
