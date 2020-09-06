package config

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
	// set the maximum number of peers (non-autopeering)
	CfgPeeringMaxPeers = "maxPeers"
	// set the URLs and IP addresses of peers
	CfgPeers = "peers"
	// sets a list of static peers, this is only used for CLI flags
	CfgPeersList = "peerslist"

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
	configFlagSet.Bool(CfgNetPreferIPv6, false, "defines if IPv6 is preferred for peers added through the API")
	configFlagSet.String(CfgNetGossipBindAddress, "0.0.0.0:15600", "the bind address of the gossip TCP server")
	configFlagSet.Int(CfgNetGossipReconnectAttemptIntervalSeconds, 60, "the number of seconds to wait before trying to reconnect to a disconnected peer")

	// peering
	peeringFlagSet.Bool(CfgPeeringAcceptAnyConnection, false, "enable inbound connections from unknown peers")
	peeringFlagSet.Int(CfgPeeringMaxPeers, 5, "set the maximum number of peers (non-autopeering)")
	PeeringConfig.SetDefault(CfgPeers, []PeerConfig{})

	// this is added to the configFlagSet on purpose, because it should not be added to the peering.json after neighbors changed
	configFlagSet.StringSlice(CfgPeersList, []string{}, "a list of peers to connect to")

	// autopeering
	configFlagSet.StringSlice(CfgNetAutopeeringEntryNodes, []string{
		"46CstniGgfWMdAySiWuS7bVfugwuHZCUQKVaC4Y34EYJ@enter.hornet.zone:14626",
		"EkSLZ4uvSTED1x6KaGzqxoGxjbytt2rPVfbJk1LRLCGL@enter.manapotion.io:18626",
		"2GHfjJhTqRaKCGBJJvS5RWty61XhjX7FtbVDhg7s8J1x@entrynode.tanglebay.org:14626",
		"iotaMk9Rg8wWo1DDeG7fwV9iJ41hvkwFX8w6MyTQgDu@enter.thetangle.org:14627",
	}, "list of autopeering entry nodes to use")
	configFlagSet.String(CfgNetAutopeeringBindAddr, "0.0.0.0:14626", "bind address for global services such as autopeering and gossip")
	configFlagSet.String(CfgNetAutopeeringSeed, "", "private key seed used to derive the node identity; optional Base64 encoded 256-bit string")
	configFlagSet.Bool(CfgNetAutopeeringRunAsEntryNode, false, "whether the node should act as an autopeering entry node")
	configFlagSet.Int(CfgNetAutopeeringInboundPeers, 2, "the number of inbound autopeers")
	configFlagSet.Int(CfgNetAutopeeringOutboundPeers, 2, "the number of outbound autopeers")
	configFlagSet.Int(CfgNetAutopeeringSaltLifetime, 30, "lifetime (in minutes) of the private and public local salt")
	configFlagSet.Int(CfgNetAutopeeringMaxDroppedPacketsPercentage, 0, "maximum percentage of dropped packets in one minute before an autopeered neighbor gets dropped (0 = disable)")
}
