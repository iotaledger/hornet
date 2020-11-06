package gossip

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// Defines the maximum amount of unknown peers a gossip protocol connection is established to.
	CfgP2PGossipUnknownPeersLimit = "p2p.gossipUnknownPeersLimit"
)

func init() {
	cli.ConfigFlagSet.Int(CfgP2PGossipUnknownPeersLimit, 4, "maximum amount of unknown peers a gossip protocol connection is established to")
}
