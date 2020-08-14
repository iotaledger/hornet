package config

import (
	flag "github.com/spf13/pflag"
)

// PeerConfig holds the initial information about peers.
type PeerConfig struct {
	ID         string `json:"identity" mapstructure:"identity"`
	Alias      string `json:"alias" mapstructure:"alias"`
	PreferIPv6 bool   `json:"preferIPv6" mapstructure:"preferIPv6"`
}

const (
	// Defines if IPv6 is preferred for peers added through the API
	CfgNetPreferIPv6 = "network.preferIPv6"
	// the bind address of the gossip TCP server
	CfgNetGossipBindAddress = "network.gossip.bindAddress"
	// the number of seconds to wait before trying to reconnect to a disconnected peer
	CfgNetGossipReconnectAttemptIntervalSeconds = "network.gossip.reconnectAttemptIntervalSeconds"

	// enable inbound connections from unknown peers
	CfgPeeringAcceptAnyConnection = "acceptAnyConnection"
	// set the maximum number of peers
	CfgPeeringMaxPeers = "maxPeers"
	// set the URLs and IP addresses of peers
	CfgPeers = "peers"

	// list of autopeering entry nodes to use
	CfgNetAutopeeringEntryNodes = "network.autopeering.entryNodes"
	// bind address for global services such as autopeering and gossip
	CfgNetAutopeeringBindAddr = "network.autopeering.bindAddress"
	// private key seed used to derive the node identity; optional Base64 encoded 256-bit string
	CfgNetAutopeeringSeed = "network.autopeering.seed"
	// whether the node should act as an autopeering entry node
	CfgNetAutopeeringRunAsEntryNode = "network.autopeering.runAsEntryNode"
	// the number of inbound autopeers
	CfgNetAutopeeringInboundPeers = "network.autopeering.inboundPeers"
	// the number of outbound autopeers
	CfgNetAutopeeringOutboundPeers = "network.autopeering.outboundPeers"
	// lifetime (in minutes) of the private and public local salt
	CfgNetAutopeeringSaltLifetime = "network.autopeering.saltLifetime"
	// maximum percentage of dropped packets in one minute before an autopeered neighbor gets dropped
	CfgNetAutopeeringMaxDroppedPacketsPercentage = "network.autopeering.maxDroppedPacketsPercentage"
)

func init() {

	// gossip
	flag.Bool(CfgNetPreferIPv6, false, "defines if IPv6 is preferred for peers added through the API")
	flag.String(CfgNetGossipBindAddress, "0.0.0.0:15600", "the bind address of the gossip TCP server")
	flag.Int(CfgNetGossipReconnectAttemptIntervalSeconds, 60, "the number of seconds to wait before trying to reconnect to a disconnected peer")

	// peering
	flag.Bool(CfgPeeringAcceptAnyConnection, false, "enable inbound connections from unknown peers")
	flag.Int(CfgPeeringMaxPeers, 5, "set the maximum number of peers")
	PeeringConfig.SetDefault(CfgPeers, []PeerConfig{})

	// autopeering
	flag.StringSlice(CfgNetAutopeeringEntryNodes, []string{
		"46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ@enter.hornet.zone:14626",
		"EkSLZ4uvSTED1x6KaGzqxoGxjbytt2rPVfbJk1LRLCGL@enter.manapotion.io:18626",
		"2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x@entrynode.tanglebay.org:14626",
		"iotaMk9Rg8wWo1DDeG7fwV9iJ41hvkwFX8w6MyTQgDu@enter.thetangle.org:14627",
	}, "list of autopeering entry nodes to use")
	flag.String(CfgNetAutopeeringBindAddr, "0.0.0.0:14626", "bind address for global services such as autopeering and gossip")
	flag.String(CfgNetAutopeeringSeed, "", "private key seed used to derive the node identity; optional Base64 encoded 256-bit string")
	flag.Bool(CfgNetAutopeeringRunAsEntryNode, false, "whether the node should act as an autopeering entry node")
	flag.Int(CfgNetAutopeeringInboundPeers, 2, "the number of inbound autopeers")
	flag.Int(CfgNetAutopeeringOutboundPeers, 2, "the number of outbound autopeers")
	flag.Int(CfgNetAutopeeringSaltLifetime, 30, "lifetime (in minutes) of the private and public local salt")
	flag.Int(CfgNetAutopeeringMaxDroppedPacketsPercentage, 0, "maximum percentage of dropped packets in one minute before an autopeered neighbor gets dropped (0 = disable)")
}
