package main

import (
	"github.com/gohornet/hornet/core/app"
	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/core/gracefulshutdown"
	"github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/core/pow"
	"github.com/gohornet/hornet/core/profile"
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/plugins/autopeering"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/debug"
	"github.com/gohornet/hornet/plugins/faucet"
	"github.com/gohornet/hornet/plugins/inx"
	"github.com/gohornet/hornet/plugins/participation"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/prometheus"
	"github.com/gohornet/hornet/plugins/receipt"
	"github.com/gohornet/hornet/plugins/restapi"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/gohornet/hornet/plugins/versioncheck"
	"github.com/gohornet/hornet/plugins/warpsync"
)

func main() {
	node.Run(
		node.WithInitPlugin(app.InitPlugin),
		node.WithCorePlugins([]*node.CorePlugin{
			profile.CorePlugin,
			protocfg.CorePlugin,
			gracefulshutdown.CorePlugin,
			database.CorePlugin,
			pow.CorePlugin,
			p2p.CorePlugin,
			gossip.CorePlugin,
			tangle.CorePlugin,
			snapshot.CorePlugin,
		}...),
		node.WithPlugins([]*node.Plugin{
			profiling.Plugin,
			versioncheck.Plugin,
			restapi.Plugin,
			restapiv2.Plugin,
			autopeering.Plugin,
			warpsync.Plugin,
			urts.Plugin,
			dashboard.Plugin,
			spammer.Plugin,
			receipt.Plugin,
			prometheus.Plugin,
			inx.Plugin,
			debug.Plugin,
			faucet.Plugin,
			participation.Plugin,
		}...),
	)
}
