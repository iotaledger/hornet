package autopeering

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgNetAutopeeringBindAddr is bind address for autopeering.
	CfgNetAutopeeringBindAddr = "p2p.autopeering.bindAddress"
	// CfgNetAutopeeringEntryNodes list of autopeering entry nodes to use.
	CfgNetAutopeeringEntryNodes = "p2p.autopeering.entryNodes"
	// CfgNetAutopeeringEntryNodesPreferIPv6 defines if connecting over IPv6 is preferred for entry nodes.
	CfgNetAutopeeringEntryNodesPreferIPv6 = "p2p.autopeering.entryNodesPreferIPv6"
	// CfgNetAutopeeringRunAsEntryNode whether the node should act as an autopeering entry node.
	CfgNetAutopeeringRunAsEntryNode = "p2p.autopeering.runAsEntryNode"
	// CfgNetAutopeeringInboundPeers the number of inbound autopeers.
	CfgNetAutopeeringInboundPeers = "p2p.autopeering.inboundPeers"
	// CfgNetAutopeeringOutboundPeers the number of outbound autopeers.
	CfgNetAutopeeringOutboundPeers = "p2p.autopeering.outboundPeers"
	// CfgNetAutopeeringSaltLifetime lifetime of the private and public local salt.
	CfgNetAutopeeringSaltLifetime = "p2p.autopeering.saltLifetime"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgNetAutopeeringBindAddr, "0.0.0.0:14626", "bind address for autopeering")
			fs.StringSlice(CfgNetAutopeeringEntryNodes, []string{}, "list of autopeering entry nodes to use")
			fs.Bool(CfgNetAutopeeringEntryNodesPreferIPv6, false, "defines if connecting over IPv6 is preferred for entry nodes")
			fs.Bool(CfgNetAutopeeringRunAsEntryNode, false, "whether the node should act as an autopeering entry node")
			fs.Int(CfgNetAutopeeringInboundPeers, 2, "the number of inbound autopeers")
			fs.Int(CfgNetAutopeeringOutboundPeers, 2, "the number of outbound autopeers")
			fs.Duration(CfgNetAutopeeringSaltLifetime, 2*time.Hour, "lifetime of the private and public local salt")
			return fs
		}(),
	},
	Masked: nil,
}
