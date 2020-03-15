package main

import (
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/plugins/autopeering"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/database"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/gracefulshutdown"
	"github.com/gohornet/hornet/plugins/graph"
	"github.com/gohornet/hornet/plugins/metrics"
	"github.com/gohornet/hornet/plugins/monitor"
	"github.com/gohornet/hornet/plugins/mqtt"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/snapshot"
	"github.com/gohornet/hornet/plugins/spa"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/tipselection"
	"github.com/gohornet/hornet/plugins/webapi"
	"github.com/gohornet/hornet/plugins/zeromq"
)

func main() {
	cli.PrintVersion()
	cli.ParseConfig()

	plugins := []*node.Plugin{
		cli.PLUGIN,
		gracefulshutdown.PLUGIN,
		database.PLUGIN,
		autopeering.PLUGIN,
	}

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		plugins = append(plugins, []*node.Plugin{
			gossip.PLUGIN,
			tangle.PLUGIN,
			tipselection.PLUGIN,
			metrics.PLUGIN,
			profiling.PLUGIN,
			snapshot.PLUGIN,
			webapi.PLUGIN,
			spa.PLUGIN,
			zeromq.PLUGIN,
			mqtt.PLUGIN,
			graph.PLUGIN,
			monitor.PLUGIN,
			spammer.PLUGIN,
		}...)
	}

	node.Run(node.Plugins(plugins...))
}
