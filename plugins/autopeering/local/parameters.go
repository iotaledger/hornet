package local

import (
	"github.com/gohornet/hornet/packages/parameter"
)

const (
	CFG_BIND              = "network.bindAddress"
	CFG_EXTERNAL          = "network.externalAddress"
	CFG_PORT              = "autopeering.port"
	CFG_SEED              = "autopeering.seed"
	CFG_ACT_AS_ENTRY_NODE = "autopeering.actAsEntryNode"
)

func init() {
	// "bind address for global services such as autopeering and gossip"
	parameter.NodeConfig.SetDefault(CFG_BIND, "0.0.0.0")

	// "external IP address under which the node is reachable; or 'auto' to determine it automatically"
	parameter.NodeConfig.SetDefault(CFG_EXTERNAL, "auto")

	// "UDP port for incoming peering requests"
	parameter.NodeConfig.SetDefault(CFG_PORT, 14626)

	// "private key seed used to derive the node identity; optional Base64 encoded 256-bit string"
	parameter.NodeConfig.SetDefault(CFG_SEED, nil)

	// "whether the node should act as an autopeering entry node"
	parameter.NodeConfig.SetDefault(CFG_ACT_AS_ENTRY_NODE, false)
}
