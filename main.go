package main

import (
	"github.com/iotaledger/hornet/core/app"
	"github.com/iotaledger/hornet/core/database"
	"github.com/iotaledger/hornet/core/gossip"
	"github.com/iotaledger/hornet/core/gracefulshutdown"
	"github.com/iotaledger/hornet/core/p2p"
	"github.com/iotaledger/hornet/core/pow"
	"github.com/iotaledger/hornet/core/profile"
	"github.com/iotaledger/hornet/core/protocfg"
	"github.com/iotaledger/hornet/core/snapshot"
	"github.com/iotaledger/hornet/core/tangle"
	"github.com/iotaledger/hornet/pkg/node"
	"github.com/iotaledger/hornet/plugins/autopeering"
	"github.com/iotaledger/hornet/plugins/coordinator"
	"github.com/iotaledger/hornet/plugins/dashboard"
	"github.com/iotaledger/hornet/plugins/debug"
	"github.com/iotaledger/hornet/plugins/faucet"
	"github.com/iotaledger/hornet/plugins/migrator"
	"github.com/iotaledger/hornet/plugins/mqtt"
	"github.com/iotaledger/hornet/plugins/participation"
	"github.com/iotaledger/hornet/plugins/profiling"
	"github.com/iotaledger/hornet/plugins/prometheus"
	"github.com/iotaledger/hornet/plugins/receipt"
	"github.com/iotaledger/hornet/plugins/restapi"
	restapiv1 "github.com/iotaledger/hornet/plugins/restapi/v1"
	"github.com/iotaledger/hornet/plugins/spammer"
	"github.com/iotaledger/hornet/plugins/urts"
	"github.com/iotaledger/hornet/plugins/versioncheck"
	"github.com/iotaledger/hornet/plugins/warpsync"
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
			restapiv1.Plugin,
			autopeering.Plugin,
			warpsync.Plugin,
			urts.Plugin,
			dashboard.Plugin,
			spammer.Plugin,
			mqtt.Plugin,
			coordinator.Plugin,
			migrator.Plugin,
			receipt.Plugin,
			prometheus.Plugin,
			debug.Plugin,
			faucet.Plugin,
			participation.Plugin,
		}...),
	)
}
