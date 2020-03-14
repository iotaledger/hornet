package config

// NeighborConfig struct
type NeighborConfig struct {
	Identity   string `mapstructure:"identity"`
	Alias      string `mapstructure:"alias"`
	PreferIPv6 bool   `mapstructure:"preferIPv6"`
}

const (
	// Defines if IPv6 is preferred for neighbors added through the API
	CfgNetPreferIPv6 = "network.preferIPv6"
	// the bind address of the gossip TCP server
	CfgNetGossipBindAddress = "network.gossip.bindAddress"
	// the number of seconds to wait before trying to reconnect to a disconnected neighbor
	CfgNetGossipReconnectAttemptIntervalSeconds = "network.gossip.reconnectAttemptIntervalSeconds"

	// enable inbound connections from unknown neighbors
	CfgNeighborsAcceptAnyNeighborConnection = "acceptAnyNeighborConnection"
	// set the maximum number of neighbors
	CfgNeighborsMaxNeighbors = "maxNeighbors"
	// set the URLs and IP addresses of neighbors
	CfgNeighbors = "neighbors"

	// list of autopeering entry nodes to use
	CfgNetAutopeeringEntryNodes = "network.autopeering.entryNodes"
	// bind address for global services such as autopeering and gossip
	CfgNetAutopeeringBindAddr = "network.autopeering.bindAddress"
	// external IP address under which the node is reachable; or 'auto' to determine it automatically
	CfgNetAutopeeringExternalAddr = "network.autopeering.externalAddress"
	// private key seed used to derive the node identity; optional Base64 encoded 256-bit string
	CfgNetAutopeeringSeed = "network.autopeering.seed"
	// whether the node should act as an autopeering entry node
	CfgNetAutopeeringRunAsEntryNode = "network.autopeering.runAsEntryNode"
)

func init() {
	// gossip
	NodeConfig.SetDefault(CfgNetPreferIPv6, false)
	NodeConfig.SetDefault(CfgNetGossipBindAddress, "0.0.0.0")
	NodeConfig.SetDefault(CfgNetGossipReconnectAttemptIntervalSeconds, 60)

	// neighbors
	NeighborsConfig.SetDefault(CfgNeighborsAcceptAnyNeighborConnection, false)
	NeighborsConfig.SetDefault(CfgNeighborsMaxNeighbors, 5)
	NeighborsConfig.SetDefault(CfgNeighbors, []NeighborConfig{
		{
			Identity:   "example.neighbor.com:15600",
			Alias:      "Example Neighbor",
			PreferIPv6: false,
		},
	})

	// autopeering
	NodeConfig.SetDefault(CfgNetAutopeeringEntryNodes, []string{
		"LehlDBPJ6kfcfLOK6kAU4nD7B/BdR7SJhai7yFCbCCM=@enter.hornet.zone:14626",
	})
	NodeConfig.SetDefault(CfgNetAutopeeringBindAddr, "0.0.0.0:14626")
	NodeConfig.SetDefault(CfgNetAutopeeringExternalAddr, "auto")
	NodeConfig.SetDefault(CfgNetAutopeeringSeed, nil)
	NodeConfig.SetDefault(CfgNetAutopeeringRunAsEntryNode, false)
}
