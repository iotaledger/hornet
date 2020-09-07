package main

import (
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/toolset"
	"github.com/gohornet/hornet/plugins/autopeering"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/database"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gracefulshutdown"
	"github.com/gohornet/hornet/plugins/metrics"
	"github.com/gohornet/hornet/plugins/mqtt"
	"github.com/gohornet/hornet/plugins/peering"
	"github.com/gohornet/hornet/plugins/pow"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/prometheus"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/gohornet/hornet/plugins/warpsync"
	"github.com/gohornet/hornet/plugins/webapi"
	"github.com/gohornet/hornet/plugins/zmq"
)

func main() {
	cli.HideConfigFlags()
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
		autopeering.PLUGIN,
		webapi.PLUGIN,
	}

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		plugins = append(plugins, []*node.Plugin{
			pow.PLUGIN,
			gossip.PLUGIN,
			tangle.PLUGIN,
			peering.PLUGIN,
			warpsync.PLUGIN,
			urts.PLUGIN,
			metrics.PLUGIN,
			snapshot.PLUGIN,
			dashboard.PLUGIN,
			zmq.PLUGIN,
			mqtt.PLUGIN,
			spammer.PLUGIN,
			coordinator.PLUGIN,
			prometheus.PLUGIN,
		}...)
	}

	node.Run(node.Plugins(plugins...))
}
