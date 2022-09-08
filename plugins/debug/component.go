package debug

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	restapipkg "github.com/iotaledger/hornet/v2/pkg/restapi"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/plugins/restapi"
	"github.com/iotaledger/inx-app/httpserver"
)

const (
	// RouteDebugSolidifier is the debug route to manually trigger the solidifier.
	// POST triggers the solidifier.
	RouteDebugSolidifier = "/solidifier"

	// RouteDebugOutputs is the debug route for getting all output IDs.
	// GET returns the outputIDs for all outputs.
	RouteDebugOutputs = "/outputs"

	// RouteDebugOutputsUnspent is the debug route for getting all unspent output IDs.
	// GET returns the outputIDs for all unspent outputs.
	RouteDebugOutputsUnspent = "/outputs/unspent"

	// RouteDebugOutputsSpent is the debug route for getting all spent output IDs.
	// GET returns the outputIDs for all spent outputs.
	RouteDebugOutputsSpent = "/outputs/spent"

	// RouteDebugMilestoneDiffs is the debug route for getting a milestone diff by it's milestoneIndex.
	// GET returns the utxo diff (new outputs & spents) for the milestone index.
	RouteDebugMilestoneDiffs = "/ms-diff/:" + restapipkg.ParameterMilestoneIndex

	// RouteDebugRequests is the debug route for getting all pending requests.
	// GET returns a list of all pending requests.
	RouteDebugRequests = "/requests"

	// RouteDebugBlockCone is the debug route for traversing a cone of a block.
	// it traverses the parents of a block until they reference an older milestone than the start block.
	// GET returns the path of this traversal and the "entry points".
	RouteDebugBlockCone = "/block-cones/:" + restapipkg.ParameterBlockID
)

func init() {
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:      "Debug",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
		},
		IsEnabled: func() bool {
			return ParamsDebug.Enabled
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In
	Storage          *storage.Storage
	SyncManager      *syncmanager.SyncManager
	Tangle           *tangle.Tangle
	RequestQueue     gossip.RequestQueue
	UTXOManager      *utxo.Manager
	RestRouteManager *restapi.RestRouteManager `optional:"true"`
}

func configure() error {
	// check if RestAPI plugin is disabled
	if Plugin.App().IsPluginSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the Debug plugin")
	}

	routeGroup := deps.RestRouteManager.AddRoute("debug/v1")

	routeGroup.POST(RouteDebugSolidifier, func(c echo.Context) error {
		deps.Tangle.TriggerSolidifier()

		return c.NoContent(http.StatusNoContent)
	})

	routeGroup.GET(RouteDebugOutputs, func(c echo.Context) error {
		resp, err := outputsIDs(c)
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugOutputsUnspent, func(c echo.Context) error {
		resp, err := unspentOutputsIDs(c)
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugOutputsSpent, func(c echo.Context) error {
		resp, err := spentOutputsIDs(c)
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugMilestoneDiffs, func(c echo.Context) error {
		resp, err := milestoneDiff(c)
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugRequests, func(c echo.Context) error {
		resp, err := requests(c)
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteDebugBlockCone, func(c echo.Context) error {
		resp, err := blockCone(c)
		if err != nil {
			return err
		}

		return httpserver.JSONResponse(c, http.StatusOK, resp)
	})

	return nil
}
