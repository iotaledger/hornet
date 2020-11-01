package main

import (
	"github.com/gohornet/hornet/plugins/p2p"
	"github.com/gohornet/hornet/plugins/p2pdisc"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/toolset"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/database"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gracefulshutdown"
	"github.com/gohornet/hornet/plugins/metrics"
	"github.com/gohornet/hornet/plugins/mqtt"
	"github.com/gohornet/hornet/plugins/pow"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/prometheus"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/gohornet/hornet/plugins/warpsync"
)

func main() {
	cli.ParseFlags()
	cli.PrintVersion()
	cli.ParseConfig()
	toolset.HandleTools()
	cli.PrintConfig()

	plugins := []*node.Plugin{
		cli.PLUGIN,
		gracefulshutdown.PLUGIN,
		profiling.PLUGIN,
		database.PLUGIN,
		restapi.PLUGIN,
		pow.PLUGIN,
		p2p.PLUGIN,
		p2pdisc.PLUGIN,
		gossip.PLUGIN,
		tangle.PLUGIN,
		warpsync.PLUGIN,
		urts.PLUGIN,
		metrics.PLUGIN,
		snapshot.PLUGIN,
		dashboard.PLUGIN,
		spammer.PLUGIN,
		mqtt.PLUGIN,
		coordinator.PLUGIN,
		prometheus.PLUGIN,
	}

	node.Run(node.Plugins(plugins...))
}
