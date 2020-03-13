package gossip

import (
	"github.com/gohornet/hornet/packages/config"
)

// NeighborConfig struct
type NeighborConfig struct {
	Identity   string `mapstructure:"identity"`
	Alias      string `mapstructure:"alias"`
	PreferIPv6 bool   `mapstructure:"preferIPv6"`
}

func init() {
	// "Defines if IPv6 is preferred for neighbors added through the API"
	config.NodeConfig.SetDefault(config.CfgNetPreferIPv6, false)

	// "Bind the TCP server socket for the gossip protocol to an address"
	config.NodeConfig.SetDefault(config.CfgNetGossipBindAddress, "0.0.0.0")

	// "Set the number of seconds to wait before trying to reconnect to a disconnected neighbor"
	config.NodeConfig.SetDefault(config.CfgNetGossipReconnectAttemptIntervalSeconds, 60)

	// "The address of the coordinator"
	config.NodeConfig.SetDefault(config.CfgMilestoneCoordinator, "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9")

	// "The security level used in coordinator signatures"
	config.NodeConfig.SetDefault(config.CfgMilestoneCoordinatorSecurityLevel, 2)

	// "The depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message."
	config.NodeConfig.SetDefault(config.CfgMilestoneNumberOfKeysInAMilestone, 23)

	// "The minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash. Increasing this number by 1 will result in proof of work that is 3 times as hard."
	config.NodeConfig.SetDefault(config.CfgProtocolMWM, 14)

	///////////////////////////////////////// NeighborsConfig /////////////////////////////////////////

	// "Enable new connections from unknown neighbors"
	config.NeighborsConfig.SetDefault(config.CfgNeighborsAcceptAnyNeighborConnection, false)

	// "Set the maximum number of neighbors"
	config.NeighborsConfig.SetDefault(config.CfgNeighborsMaxNeighbors, 5)

	// "Set the URLs and IP addresses of neighbors"
	config.NeighborsConfig.SetDefault(config.CfgNeighbors, []NeighborConfig{
		{
			Identity:   "example1.neighbor.com:15600",
			Alias:      "Example Neighbor 1",
			PreferIPv6: false,
		},
	})
}
