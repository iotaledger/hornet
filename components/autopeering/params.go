package autopeering

import (
	"time"

	"github.com/iotaledger/hive.go/app"
)

// ParametersAutopeering contains the definition of the parameters used by autopeering.
type ParametersAutopeering struct {
	// Enabled defines whether the autopeering plugin is enabled.
	Enabled bool `default:"false" usage:"whether the autopeering plugin is enabled"`
	// BindAddress is bind address for autopeering.
	BindAddress string `default:"0.0.0.0:14626" usage:"bind address for autopeering"`
	// EntryNodes list of autopeering entry nodes to use.
	EntryNodes []string `default:"/dns/entry-hornet-0.h.stardust-mainnet.iotaledger.net/udp/14626/autopeering/K4cHM64KxzYJ5ZB2a9P3stJUHjvQDh4bzhCw49xDowi,/dns/entry-hornet-1.h.stardust-mainnet.iotaledger.net/udp/14626/autopeering/8UbVu5MjRZH2c9fnEdpfPvd7qqDgrVFsNsvc933FuMTm" usage:"list of autopeering entry nodes to use"`
	// EntryNodesPreferIPv6 defines if connecting over IPv6 is preferred for entry nodes.
	EntryNodesPreferIPv6 bool `default:"false" usage:"defines if connecting over IPv6 is preferred for entry nodes"`
	// RunAsEntryNode whether the node should act as an autopeering entry node.
	RunAsEntryNode bool `default:"false" usage:"whether the node should act as an autopeering entry node"`
	// InboundPeers the number of inbound autopeers.
	InboundPeers int `default:"2" usage:"the number of inbound autopeers"`
	// OutboundPeers the number of outbound autopeers.
	OutboundPeers int `default:"2" usage:"the number of outbound autopeers"`
	// SaltLifetime lifetime of the private and public local salt.
	SaltLifetime time.Duration `default:"2h" usage:"lifetime of the private and public local salt"`
}

var ParamsAutopeering = &ParametersAutopeering{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"p2p.autopeering": ParamsAutopeering,
	},
	Masked: nil,
}
