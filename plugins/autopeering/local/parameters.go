package local

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// bind address for global services such as autopeering and gossip
	config.NodeConfig.SetDefault(config.CfgNetAutopeeringBindAddr, "0.0.0.0:14626")

	// external IP address under which the node is reachable; or 'auto' to determine it automatically
	config.NodeConfig.SetDefault(config.CfgNetAutopeeringExternalAddr, "auto")

	// private key seed used to derive the node identity; optional Base64 encoded 256-bit string
	config.NodeConfig.SetDefault(config.CfgNetAutopeeringSeed, nil)

	// whether the node should act as an autopeering entry node
	config.NodeConfig.SetDefault(config.CfgNetAutopeeringRunAsEntryNode, false)
}
