package p2pdisc

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgP2PDiscAdvertiseInterval defines the interval at which the node advertises itself on the DHT for peer discovery.
	CfgP2PDiscAdvertiseInterval = "p2pdisc.advertiseInterval"
	// CfgP2PDiscMaxDiscoveredPeerConns defines the max. amount of peers to be connected to which
	// were discovered via the DHT rendezvous.
	CfgP2PDiscMaxDiscoveredPeerConns = "p2pdisc.maxDiscoveredPeerConns"
	// CfgP2PDiscRendezvousPoint defines the rendezvous string for advertising on the DHT
	// that the node wants to peer with others.
	CfgP2PDiscRendezvousPoint = "p2pdisc.rendezvousPoint"
	// CfgP2PDiscRoutingTableRefreshPeriod defines the routing table refresh period.
	CfgP2PDiscRoutingTableRefreshPeriod = "p2pdisc.routingTableRefreshPeriod"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Duration(CfgP2PDiscAdvertiseInterval, 30*time.Second, "the interval at which the node advertises itself on the DHT for peer discovery")
			fs.Int(CfgP2PDiscMaxDiscoveredPeerConns, 4, "the max. amount of peers to be connected to which were discovered via the DHT rendezvous")
			fs.String(CfgP2PDiscRendezvousPoint, "between-two-vertices", "the rendezvous string for advertising on the DHT that the node wants to peer with others")
			fs.Duration(CfgP2PDiscRoutingTableRefreshPeriod, 60*time.Second, "the routing table refresh period")
			return fs
		}(),
	},
	Masked: nil,
}
