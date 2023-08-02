package main

import (
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/toolset"
	"github.com/iotaledger/hornet/plugins/autopeering"
	"github.com/iotaledger/hornet/plugins/cli"
	"github.com/iotaledger/hornet/plugins/coordinator"
	"github.com/iotaledger/hornet/plugins/curl"
	"github.com/iotaledger/hornet/plugins/dashboard"
	"github.com/iotaledger/hornet/plugins/database"
	"github.com/iotaledger/hornet/plugins/gossip"
	"github.com/iotaledger/hornet/plugins/gracefulshutdown"
	"github.com/iotaledger/hornet/plugins/metrics"
	"github.com/iotaledger/hornet/plugins/mqtt"
	"github.com/iotaledger/hornet/plugins/peering"
	"github.com/iotaledger/hornet/plugins/pow"
	"github.com/iotaledger/hornet/plugins/profiling"
	"github.com/iotaledger/hornet/plugins/prometheus"
	"github.com/iotaledger/hornet/plugins/snapshot"
	"github.com/iotaledger/hornet/plugins/tangle"
	"github.com/iotaledger/hornet/plugins/urts"
	"github.com/iotaledger/hornet/plugins/warpsync"
	"github.com/iotaledger/hornet/plugins/webapi"
	"github.com/iotaledger/hornet/plugins/zmq"
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
		curl.PLUGIN,
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
			coordinator.PLUGIN,
			prometheus.PLUGIN,
		}...)
	}

	node.Run(node.Plugins(plugins...))
}
