package config

const (
	// CfgP2PDiscAdvertiseIntervalSec defines the interval at which the node advertises itself on the DHT for peer discovery.
	CfgP2PDiscAdvertiseIntervalSec = "p2pdisc.advertiseIntervalSec"
	// CfgP2PDiscMaxDiscoveredPeerConns defines the max. amount of peers to be connected to which
	// were discovered via the DHT rendezvous.
	CfgP2PDiscMaxDiscoveredPeerConns = "p2pdisc.maxDiscoveredPeerConns"
	// CfgP2PDiscRendezvousPoint defines the rendezvous string for advertising on the DHT
	// that the node wants to peer with others.
	CfgP2PDiscRendezvousPoint = "p2pdisc.rendezvousPoint"
	// CfgP2PDiscRoutingTableRefreshPeriodSec defines the routing table refresh period.
	CfgP2PDiscRoutingTableRefreshPeriodSec = "p2pdisc.routingTableRefreshPeriodSec"
)

func init() {
	configFlagSet.Int(CfgP2PDiscAdvertiseIntervalSec, 30, "defines the interval at which the node advertises itself on the DHT for peer discovery")
	configFlagSet.Int(CfgP2PDiscMaxDiscoveredPeerConns, 4, "defines the max. amount of peers to be connected to which were discovered via the DHT rendezvous")
	configFlagSet.String(CfgP2PDiscRendezvousPoint, "between-two-vertices", "defines the rendezvous string for advertising on the DHT that the node wants to peer with others")
	configFlagSet.Int(CfgP2PDiscRoutingTableRefreshPeriodSec, 60, "defines the routing table refresh period")
}
