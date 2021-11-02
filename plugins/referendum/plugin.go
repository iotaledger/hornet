package referendum

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/partitipation"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
)

const (
	// ParameterReferendumID is used to identify a partitipation by its ID.
	ParameterReferendumID = "referendumID"

	// ParameterOutputID is used to identify an output by its ID.
	ParameterOutputID = "outputID"
)

const (

	// RouteReferendums is the route to list all referendums, returning their UUID, the partitipation name and status.
	// GET returns a list of all referendums known to the node.
	// POST creates a new vote to track
	// TODO: add query filter for payload type
	RouteReferendums = "/referendums"

	// RouteReferendum is the route to access a single partitipation by its ID.
	// GET gives a quick overview of the partitipation. This does not include the current standings.
	// DELETE removes a tracked partitipation.
	RouteReferendum = "/referendums/:" + ParameterReferendumID

	// RouteReferendumStatus is the route to access the status of a single partitipation by its ID.
	// GET returns the amount of tokens voting and the weight on each option of every question.
	RouteReferendumStatus = "/referendums/:" + ParameterReferendumID + "/status"

	// RouteOutputStatus is the route to get the vote status for a given outputID.
	// GET returns the messageID the vote was included, the starting and ending milestone index this vote was tracked.
	RouteOutputStatus = "/outputs/:" + ParameterOutputID
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Referendum",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	onUTXOOutput                     *events.Closure
	onUTXOSpent                      *events.Closure
	onConfirmedMilestoneIndexChanged *events.Closure
)

type dependencies struct {
	dig.In
	NodeConfig        *configuration.Configuration `name:"nodeConfig"`
	ReferendumManager *partitipation.ParticipationManager
	Tangle            *tangle.Tangle
	Echo              *echo.Echo
	ShutdownHandler   *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	type referendumDeps struct {
		dig.In
		Storage        *storage.Storage
		SyncManager    *syncmanager.SyncManager
		DatabasePath   string                       `name:"databasePath"`
		DatabaseEngine database.Engine              `name:"databaseEngine"`
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps referendumDeps) *partitipation.ParticipationManager {

		referendumStore, err := database.StoreWithDefaultSettings(filepath.Join(deps.DatabasePath, "partitipation"), true, deps.DatabaseEngine)
		if err != nil {
			Plugin.Panic(err)
		}

		rm, err := partitipation.NewManager(
			deps.Storage,
			deps.SyncManager,
			referendumStore,
			partitipation.WithLogger(Plugin.Logger()),
		)
		if err != nil {
			Plugin.Panic(err)
		}
		return rm
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	routeGroup := deps.Echo.Group("/api/plugins/partitipation")

	routeGroup.GET(RouteReferendums, func(c echo.Context) error {
		resp, err := getReferendums(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteReferendums, func(c echo.Context) error {

		resp, err := createReferendum(c)
		if err != nil {
			return err
		}

		c.Response().Header().Set(echo.HeaderLocation, resp.ReferendumID)
		return restapi.JSONResponse(c, http.StatusCreated, resp)
	})

	routeGroup.GET(RouteReferendum, func(c echo.Context) error {
		resp, err := getReferendum(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.DELETE(RouteReferendum, func(c echo.Context) error {
		if err := deleteReferendum(c); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})

	routeGroup.GET(RouteReferendumStatus, func(c echo.Context) error {
		resp, err := getReferendumStatus(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteOutputStatus, func(c echo.Context) error {
		resp, err := getOutputStatus(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	if err := Plugin.Node.Daemon().BackgroundWorker("Close Referendum database", func(ctx context.Context) {
		<-ctx.Done()

		Plugin.LogInfo("Syncing Referendum database to disk...")
		if err := deps.ReferendumManager.CloseDatabase(); err != nil {
			Plugin.Panicf("Syncing Referendum database to disk... failed: %s", err)
		}
		Plugin.LogInfo("Syncing Referendum database to disk... done")
	}, shutdown.PriorityCloseDatabase); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}

	configureEvents()
}

func run() {
	// create a background worker that handles the partitipation events
	if err := Plugin.Daemon().BackgroundWorker("Referendum", func(ctx context.Context) {
		Plugin.LogInfo("Starting Referendum Manager ... done")
		attachEvents()
		<-ctx.Done()
		detachEvents()
		Plugin.LogInfo("Stopping Referendum Manager ... done")
	}, shutdown.PriorityReferendum); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}

func configureEvents() {

	onUTXOOutput = events.NewClosure(func(index milestone.Index, output *utxo.Output) {
		if err := deps.ReferendumManager.ApplyNewUTXO(index, output); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("partitipation plugin hit a critical error while applying new UTXO: %s", err.Error()))
		}
	})

	onUTXOSpent = events.NewClosure(func(index milestone.Index, spent *utxo.Spent) {
		if err := deps.ReferendumManager.ApplySpentUTXO(index, spent); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("partitipation plugin hit a critical error while applying spent TXO: %s", err.Error()))
		}
	})

	onConfirmedMilestoneIndexChanged = events.NewClosure(func(index milestone.Index) {
		if err := deps.ReferendumManager.ApplyNewConfirmedMilestoneIndex(index); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("partitipation plugin hit a critical error while applying new confirmed milestone index: %s", err.Error()))
		}
	})
}

func attachEvents() {
	deps.Tangle.Events.NewUTXOOutput.Attach(onUTXOOutput)
	deps.Tangle.Events.NewUTXOSpent.Attach(onUTXOSpent)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
}

func detachEvents() {
	deps.Tangle.Events.NewUTXOOutput.Detach(onUTXOOutput)
	deps.Tangle.Events.NewUTXOSpent.Detach(onUTXOSpent)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
}
