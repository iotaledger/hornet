package gossip

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// Defines the maximum time a request stays in the request queue.
	CfgRequestsDiscardOlderThan = "requests.discardOlderThan"
	// Defines the interval the pending requests are re-enqueued.
	CfgRequestsPendingReEnqueueInterval = "requests.pendingReEnqueueInterval"
	// Defines the maximum amount of unknown peers a gossip protocol connection is established to.
	CfgP2PGossipUnknownPeersLimit = "p2p.gossip.unknownPeersLimit"
	// Defines the read timeout for subsequent reads.
	CfgP2PGossipStreamReadTimeout = "p2p.gossip.streamReadTimeout"
	// Defines the write timeout for writes to the stream.
	CfgP2PGossipStreamWriteTimeout = "p2p.gossip.streamWriteTimeout"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Duration(CfgRequestsDiscardOlderThan, 15*time.Second, "the maximum time a request stays in the request queue")
			fs.Duration(CfgRequestsPendingReEnqueueInterval, 5*time.Second, "the interval the pending requests are re-enqueued")
			fs.Int(CfgP2PGossipUnknownPeersLimit, 4, "maximum amount of unknown peers a gossip protocol connection is established to")
			fs.Duration(CfgP2PGossipStreamReadTimeout, 60*time.Second, "the read timeout for reads from the gossip stream")
			fs.Duration(CfgP2PGossipStreamWriteTimeout, 10*time.Second, "the write timeout for writes to the gossip stream")
			return fs
		}(),
	},
	Masked: nil,
}
