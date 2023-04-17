package metrics

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hornet/v2/components/restapi"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/inx-app/pkg/httpserver"
)

const (
	// RouteNodeInfoExtended is the route to get additional info about the node.
	// GET returns the extended info of the node.
	RouteNodeInfoExtended = "/info"

	// RouteDatabaseSizes is the route to get the size of the databases.
	// GET returns the sizes of the databases.
	RouteDatabaseSizes = "/database/sizes"

	// RouteGossipMetrics is the route to get metrics about gossip.
	// GET returns the gossip metrics.
	RouteGossipMetrics = "/gossip"
)

func init() {
	Component = &app.Component{
		Name:     "DashboardMetrics",
		DepsFunc: func(cDeps dependencies) { deps = cDeps },
		IsEnabled: func(c *dig.Container) bool {
			// do not enable in "autopeering entry node" mode
			// the plugin is enabled if the restapi plugin is enabled
			return components.IsAutopeeringEntryNodeDisabled(c) && restapi.ParamsRestAPI.Enabled
		},
		Configure: configure,
		Run:       run,
	}
}

var (
	Component *app.Component
	deps      dependencies
)

type dependencies struct {
	dig.In
	RestRouteManager *restapi.RestRouteManager `optional:"true"`
	AppInfo          *app.Info
	Host             host.Host
	NodeAlias        string             `name:"nodeAlias"`
	TangleDatabase   *database.Database `name:"tangleDatabase"`
	UTXODatabase     *database.Database `name:"utxoDatabase"`
	Tangle           *tangle.Tangle
}

func configure() error {
	// check if RestAPI plugin is disabled
	if !Component.App().IsComponentEnabled(restapi.Component.Identifier()) {
		Component.LogPanic("RestAPI plugin needs to be enabled to use the dashboard metrics plugin")
	}

	routeGroup := deps.RestRouteManager.AddRoute("dashboard-metrics/v1")

	routeGroup.GET(RouteNodeInfoExtended, func(c echo.Context) error {
		return httpserver.JSONResponse(c, http.StatusOK, nodeInfoExtended())
	})

	routeGroup.GET(RouteDatabaseSizes, func(c echo.Context) error {
		resp, err := databaseSizesMetrics()
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteGossipMetrics, func(c echo.Context) error {
		return httpserver.JSONResponse(c, http.StatusOK, gossipMetrics())
	})

	return nil
}

func run() error {
	if err := Component.Daemon().BackgroundWorker("DashboardMetricsUpdater", func(ctx context.Context) {
		unhook := deps.Tangle.Events.BPSMetricsUpdated.Hook(func(gossipMetrics *tangle.BPSMetrics) {
			lastGossipMetricsLock.Lock()
			defer lastGossipMetricsLock.Unlock()

			lastGossipMetrics = gossipMetrics
		}).Unhook
		defer unhook()
		<-ctx.Done()
	}, daemon.PriorityMetricsUpdater); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
