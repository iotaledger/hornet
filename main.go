package main

import (
	"github.com/gohornet/hornet/core/cli"
	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/core/gracefulshutdown"
	"github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/core/pow"
	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/toolset"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/mqtt"
	"github.com/gohornet/hornet/plugins/p2pdisc"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/prometheus"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/gohornet/hornet/plugins/warpsync"
)

func main() {

	cli.ParseFlags()
	cli.PrintVersion()
	cli.ParseConfig()
	toolset.HandleTools()
	cli.PrintConfig()

	node.Run(
		node.WithDisabledPlugins(config.NodeConfig.Strings(config.CfgNodeDisablePlugins)...),
		node.WithEnabledPlugins(config.NodeConfig.Strings(config.CfgNodeEnablePlugins)...),
		node.WithCoreModules([]*node.CoreModule{
			cli.CoreModule,
			gracefulshutdown.CoreModule,
			database.CoreModule,
			pow.CoreModule,
			p2p.CoreModule,
			gossip.CoreModule,
			tangle.CoreModule,
			snapshot.CoreModule,
		}...),
		node.WithPlugins([]*node.Plugin{
			profiling.Plugin,
			restapi.Plugin,
			p2pdisc.Plugin,
			warpsync.Plugin,
			urts.Plugin,
			dashboard.Plugin,
			spammer.Plugin,
			mqtt.Plugin,
			coordinator.Plugin,
			prometheus.Plugin,
		}...),
	)
}
