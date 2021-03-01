package gossip

import (
	"time"

	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// Defines the maximum amount of unknown peers a gossip protocol connection is established to.
	CfgP2PGossipUnknownPeersLimit = "p2p.gossipUnknownPeersLimit"
	// Defines the read timeout for subsequent reads.
	CfgGossipStreamReadTimeout = "gossip.streamReadTimeout"
	// Defines the write timeout for writes to the stream.
	CfgGossipStreamWriteTimeout = "gossip.streamWriteTimeout"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgP2PGossipUnknownPeersLimit, 4, "maximum amount of unknown peers a gossip protocol connection is established to")
			fs.Duration(CfgGossipStreamReadTimeout, 60*time.Second, "the read timeout for reads from the gossip stream")
			fs.Duration(CfgGossipStreamWriteTimeout, 10*time.Second, "the write timeout for writes to the gossip stream")
			return fs
		}(),
	},
	Masked: nil,
}
