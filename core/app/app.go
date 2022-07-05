package app

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/core/shutdown"
	"github.com/iotaledger/hive.go/app/plugins/profiling"
	"github.com/iotaledger/hornet/core/database"
	"github.com/iotaledger/hornet/core/gossip"
	"github.com/iotaledger/hornet/core/p2p"
	"github.com/iotaledger/hornet/core/pow"
	"github.com/iotaledger/hornet/core/profile"
	"github.com/iotaledger/hornet/core/protocfg"
	"github.com/iotaledger/hornet/core/pruning"
	"github.com/iotaledger/hornet/core/snapshot"
	"github.com/iotaledger/hornet/core/tangle"
	"github.com/iotaledger/hornet/pkg/toolset"
	"github.com/iotaledger/hornet/plugins/autopeering"
	"github.com/iotaledger/hornet/plugins/coreapi"
	dashboard_metrics "github.com/iotaledger/hornet/plugins/dashboard-metrics"
	"github.com/iotaledger/hornet/plugins/debug"
	"github.com/iotaledger/hornet/plugins/inx"
	"github.com/iotaledger/hornet/plugins/prometheus"
	"github.com/iotaledger/hornet/plugins/receipt"
	"github.com/iotaledger/hornet/plugins/restapi"
	"github.com/iotaledger/hornet/plugins/urts"
	"github.com/iotaledger/hornet/plugins/warpsync"
)

var (
	// Name of the app.
	Name = "HORNET"

	// Version of the app.
	Version = "2.0.0-alpha.22"
)

func App() *app.App {
	return app.New(Name, Version,
		app.WithVersionCheck("iotaledger", "hornet"),
		app.WithUsageText(fmt.Sprintf(`Usage of %s (%s %s):

Run '%s tools' to list all available tools.
		
Command line flags:
`, os.Args[0], Name, Version, os.Args[0])),
		app.WithInitComponent(InitComponent),
		app.WithCoreComponents([]*app.CoreComponent{
			profile.CoreComponent,
			protocfg.CoreComponent,
			shutdown.CoreComponent,
			database.CoreComponent,
			pow.CoreComponent,
			p2p.CoreComponent,
			gossip.CoreComponent,
			tangle.CoreComponent,
			snapshot.CoreComponent,
			pruning.CoreComponent,
		}...),
		app.WithPlugins([]*app.Plugin{
			profiling.Plugin,
			restapi.Plugin,
			coreapi.Plugin,
			autopeering.Plugin,
			warpsync.Plugin,
			urts.Plugin,
			receipt.Plugin,
			prometheus.Plugin,
			inx.Plugin,
			dashboard_metrics.Plugin,
			debug.Plugin,
		}...),
	)
}

var (
	InitComponent *app.InitComponent
)

func init() {
	InitComponent = &app.InitComponent{
		Component: &app.Component{
			Name: "App",
		},
		NonHiddenFlags: []string{
			"app.checkForUpdates",
			"app.disablePlugins",
			"app.enablePlugins",
			"app.profile",
			"config",
			"help",
			"peering",
			"profiles",
			"version",
			"deleteAll",
			"deleteDatabase",
			"revalidate",
		},
		AdditionalConfigs: []*app.ConfigurationSet{
			app.NewConfigurationSet("peering", "peering", "peeringConfigFilePath", "peeringConfig", false, true, false, "peering.json", "n"),
			app.NewConfigurationSet("profiles", "profiles", "profilesConfigFilePath", "profilesConfig", false, false, false, "profiles.json", ""),
		},
		Init: initialize,
	}
}

func initialize(_ *app.App) error {

	if toolset.ShouldHandleTools() {
		toolset.HandleTools()
		// HandleTools will call os.Exit
	}

	return nil
}
