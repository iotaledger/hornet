package autopeering

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgNetAutopeeringEntryNodes list of autopeering entry nodes to use.
	CfgNetAutopeeringEntryNodes = "p2p.autopeering.entryNodes"
	// CfgNetAutopeeringBindAddr bind address for global services such as autopeering and gossip.
	CfgNetAutopeeringBindAddr = "p2p.autopeering.bindAddress"
	// CfgNetAutopeeringRunAsEntryNode whether the node should act as an autopeering entry node.
	CfgNetAutopeeringRunAsEntryNode = "p2p.autopeering.runAsEntryNode"
	// CfgNetAutopeeringInboundPeers the number of inbound autopeers.
	CfgNetAutopeeringInboundPeers = "p2p.autopeering.inboundPeers"
	// CfgNetAutopeeringOutboundPeers the number of outbound autopeers.
	CfgNetAutopeeringOutboundPeers = "p2p.autopeering.outboundPeers"
	// CfgNetAutopeeringSaltLifetime lifetime of the private and public local salt.
	CfgNetAutopeeringSaltLifetime = "p2p.autopeering.saltLifetime"
	// CfgNetAutopeeringDatabaseDirPath is the path to the autopeering database.
	CfgNetAutopeeringDatabaseDirPath = "p2p.autopeering.db.dirPath"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.StringSlice(CfgNetAutopeeringEntryNodes, []string{}, "list of autopeering entry nodes to use")
			fs.String(CfgNetAutopeeringBindAddr, "0.0.0.0:14626", "bind address for autopeering")
			fs.Bool(CfgNetAutopeeringRunAsEntryNode, false, "whether the node should act as an autopeering entry node")
			fs.Int(CfgNetAutopeeringInboundPeers, 2, "the number of inbound autopeers")
			fs.Int(CfgNetAutopeeringOutboundPeers, 2, "the number of outbound autopeers")
			fs.Duration(CfgNetAutopeeringSaltLifetime, 2*time.Hour, "lifetime of the private and public local salt")
			fs.String(CfgNetAutopeeringDatabaseDirPath, "./p2pstore", "the directory path (excluding the name) to the autopeering database")
			return fs
		}(),
	},
	Masked: nil,
}
