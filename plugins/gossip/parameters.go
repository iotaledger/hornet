package gossip

import (
	"github.com/gohornet/hornet/packages/parameter"
)

type NeighborContainer struct {
	Neighbors []ConfigNeighbor `mapstructure:"neighbors"`
}

type ConfigNeighbor struct {
	Identity   string `json:"identity"`
	Alias      string `json:"alias"`
	PreferIPv6 bool   `json:"preferIPv6"`
}

func init() {
	// "Defines if IPv6 is preferred for neighbors added through the API"
	parameter.NodeConfig.SetDefault("network.preferIPv6", false)

	// "Bind the TCP server socket for the gossip protocol to an address"
	parameter.NodeConfig.SetDefault("network.address", "0.0.0.0")

	// "Bind the TCP server socket for the gossip protocol to a port"
	parameter.NodeConfig.SetDefault("network.port", 15600)

	// "Set the number of seconds to wait before trying to reconnect to a disconnected neighbor"
	parameter.NodeConfig.SetDefault("network.reconnectAttemptIntervalSeconds", 60)

	// "The address of the coordinator"
	parameter.NodeConfig.SetDefault("milestones.coordinator", "EQSAUZXULTTYZCLNJNTXQTQHOMOFZERHTCGTXOLTVAHKSA9OGAZDEKECURBRIXIJWNPFCQIOVFVVXJVD9")

	// "The security level used in coordinator signatures"
	parameter.NodeConfig.SetDefault("milestones.coordinatorSecurityLevel", 2)

	// "The depth of the Merkle tree which in turn determines the number of leaves (private keys) that the coordinator can use to sign a message."
	parameter.NodeConfig.SetDefault("milestones.numberOfKeysInAMilestone", 23)

	// "The minimum weight magnitude is the number of trailing 0s that must appear in the end of a transaction hash. Increasing this number by 1 will result in proof of work that is 3 times as hard."
	parameter.NodeConfig.SetDefault("protocol.mwm", 14)

	///////////////////////////////////////// NeighborsConfig /////////////////////////////////////////

	// "Enable new connections from unknown neighbors"
	parameter.NeighborsConfig.SetDefault("autoTetheringEnabled", false)

	// "Set the maximum number of neighbors"
	parameter.NeighborsConfig.SetDefault("maxNeighbors", 5)

	/*
		// "Set the URLs and IP addresses of neighbors"
		parameter.NeighborsConfig.SetDefault("neighbors", NeighborContainer{
			Neighbors: []ConfigNeighbor{
				ConfigNeighbor{
					Identity:   "example",
					Alias:      "default",
					PreferIPv6: false,
				},
			}})
	*/
}
