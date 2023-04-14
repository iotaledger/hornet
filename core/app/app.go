package app

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/components/profiling"
	"github.com/iotaledger/hive.go/app/components/shutdown"
	"github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/core/gossip"
	"github.com/iotaledger/hornet/v2/core/p2p"
	"github.com/iotaledger/hornet/v2/core/pow"
	"github.com/iotaledger/hornet/v2/core/profile"
	"github.com/iotaledger/hornet/v2/core/protocfg"
	"github.com/iotaledger/hornet/v2/core/pruning"
	"github.com/iotaledger/hornet/v2/core/snapshot"
	"github.com/iotaledger/hornet/v2/core/tangle"
	"github.com/iotaledger/hornet/v2/pkg/toolset"
	"github.com/iotaledger/hornet/v2/plugins/autopeering"
	"github.com/iotaledger/hornet/v2/plugins/coreapi"
	dashboard_metrics "github.com/iotaledger/hornet/v2/plugins/dashboard-metrics"
	"github.com/iotaledger/hornet/v2/plugins/debug"
	"github.com/iotaledger/hornet/v2/plugins/inx"
	"github.com/iotaledger/hornet/v2/plugins/prometheus"
	"github.com/iotaledger/hornet/v2/plugins/receipt"
	"github.com/iotaledger/hornet/v2/plugins/restapi"
	"github.com/iotaledger/hornet/v2/plugins/urts"
	"github.com/iotaledger/hornet/v2/plugins/warpsync"
)

var (
	// Name of the app.
	Name = "HORNET"

	// Version of the app.
	Version = "2.0.0-rc.5"
)

func App() *app.App {
	return app.New(Name, Version,
		app.WithVersionCheck("iotaledger", "hornet"),
		app.WithUsageText(fmt.Sprintf(`Usage of %s (%s %s):

Run '%s tools' to list all available tools.
		
Command line flags:
`, os.Args[0], Name, Version, os.Args[0])),
		app.WithInitComponent(InitComponent),
		app.WithComponents(
			profile.Component,
			protocfg.Component,
			shutdown.Component,
			database.Component,
			pow.Component,
			p2p.Component,
			gossip.Component,
			tangle.Component,
			snapshot.Component,
			pruning.Component,
			profiling.Component,
			restapi.Component,
			coreapi.Component,
			autopeering.Component,
			warpsync.Component,
			urts.Component,
			receipt.Component,
			prometheus.Component,
			inx.Component,
			dashboard_metrics.Component,
			debug.Component,
		),
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
