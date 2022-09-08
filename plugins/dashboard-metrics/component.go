package metrics

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/plugins/restapi"
	"github.com/iotaledger/inx-app/httpserver"
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
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:      "DashboardMetrics",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
			Run:       run,
		},
		IsEnabled: func() bool {
			// the plugin is enabled if the restapi plugin is enabled
			return restapi.ParamsRestAPI.Enabled
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies
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
	if Plugin.App().IsPluginSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the dashboard metrics plugin")
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

	onBPSMetricsUpdated := events.NewClosure(func(gossipMetrics *tangle.BPSMetrics) {
		lastGossipMetricsLock.Lock()
		defer lastGossipMetricsLock.Unlock()

		lastGossipMetrics = gossipMetrics
	})

	if err := Plugin.Daemon().BackgroundWorker("DashboardMetricsUpdater", func(ctx context.Context) {
		deps.Tangle.Events.BPSMetricsUpdated.Hook(onBPSMetricsUpdated)
		<-ctx.Done()
		deps.Tangle.Events.BPSMetricsUpdated.Detach(onBPSMetricsUpdated)
	}, daemon.PriorityMetricsUpdater); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
