package app

import (
	"fmt"
	"os"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/core/pow"
	"github.com/gohornet/hornet/core/profile"
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/toolset"

	"github.com/gohornet/hornet/plugins/autopeering"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/debug"
	"github.com/gohornet/hornet/plugins/inx"
	"github.com/gohornet/hornet/plugins/prometheus"
	"github.com/gohornet/hornet/plugins/receipt"
	"github.com/gohornet/hornet/plugins/restapi"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/gohornet/hornet/plugins/warpsync"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/core/shutdown"
	"github.com/iotaledger/hive.go/app/plugins/profiling"
)

var (
	// Name of the app.
	Name = "HORNET"

	// Version of the app.
	Version = "2.0.0-alpha13"
)

func App() *app.App {
	return app.New(Name, Version,
		app.WithVersionCheck("gohornet", "hornet"),
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
		}...),
		app.WithPlugins([]*app.Plugin{
			profiling.Plugin,
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
