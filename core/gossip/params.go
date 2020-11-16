package gossip

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// Defines the maximum amount of unknown peers a gossip protocol connection is established to.
	CfgP2PGossipUnknownPeersLimit = "p2p.gossipUnknownPeersLimit"
	// Defines the read timeout for subsequent reads in seconds.
	CfgGossipStreamReadTimeoutSec = "gossip.streamReadTimeoutSec"
	// Defines the write timeout for writes to the stream in seconds.
	CfgGossipStreamWriteTimeoutSec = "gossip.streamWriteTimeoutSec"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgP2PGossipUnknownPeersLimit, 4, "maximum amount of unknown peers a gossip protocol connection is established to")
			fs.Int(CfgGossipStreamReadTimeoutSec, 60, "the read timeout for reads from the gossip stream in seconds")
			fs.Int(CfgGossipStreamWriteTimeoutSec, 10, "the write timeout for writes to the gossip stream in seconds")
			return fs
		}(),
	},
	Masked: nil,
}
