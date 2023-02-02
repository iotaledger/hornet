package poi

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hornet/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/node"
	"github.com/iotaledger/hornet/plugins/restapi"
	restapiv1 "github.com/iotaledger/hornet/plugins/restapi/v1"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "POI",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In
	RestRouteManager *restapi.RestRouteManager `optional:"true"`
	Storage          *storage.Storage
	MilestoneManager *milestonemanager.MilestoneManager
}

func configure() {
	// check if RestAPI plugin is disabled
	if Plugin.Node.IsSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the Proof of Inclusion plugin")
	}

	restapiv1.AddFeature(Plugin.Name)

	routeGroup := deps.RestRouteManager.AddRoute("plugins/poi")

	setupRoutes(routeGroup)
}
