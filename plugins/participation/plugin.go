package participation

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/participation"
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
	// ParameterParticipationEventID is used to identify an event by its ID.
	ParameterParticipationEventID = "eventID"

	// ParameterOutputID is used to identify an output by its ID.
	ParameterOutputID = "outputID"
)

const (

	// RouteParticipationEvents is the route to list all events, returning their ID, the event name and status.
	// GET returns a list of all events known to the node.
	// POST creates a new event to track
	// TODO: add query filter for payload type
	RouteParticipationEvents = "/events"

	// RouteParticipationEvent is the route to access a single participation by its ID.
	// GET gives a quick overview of the participation. This does not include the current standings.
	// DELETE removes a tracked participation.
	RouteParticipationEvent = "/events/:" + ParameterParticipationEventID

	// RouteParticipationEventStatus is the route to access the status of a single participation by its ID.
	// GET returns the amount of tokens participating and accumulated votes for the ballot if the event contains a ballot.
	RouteParticipationEventStatus = "/events/:" + ParameterParticipationEventID + "/status"

	// RouteOutputStatus is the route to get the vote status for a given outputID.
	// GET returns the messageID the participation was included, the starting and ending milestone index this participation was tracked.
	RouteOutputStatus = "/outputs/:" + ParameterOutputID
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Participation",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
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
	NodeConfig           *configuration.Configuration `name:"nodeConfig"`
	ParticipationManager *participation.ParticipationManager
	Tangle               *tangle.Tangle
	Echo                 *echo.Echo
	ShutdownHandler      *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	type participationDeps struct {
		dig.In
		Storage        *storage.Storage
		SyncManager    *syncmanager.SyncManager
		DatabasePath   string                       `name:"databasePath"`
		DatabaseEngine database.Engine              `name:"databaseEngine"`
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps participationDeps) *participation.ParticipationManager {

		participationStore, err := database.StoreWithDefaultSettings(filepath.Join(deps.DatabasePath, "participation"), true, deps.DatabaseEngine)
		if err != nil {
			Plugin.Panic(err)
		}

		pm, err := participation.NewManager(
			deps.Storage,
			deps.SyncManager,
			participationStore,
			participation.WithLogger(Plugin.Logger()),
		)
		if err != nil {
			Plugin.Panic(err)
		}
		return pm
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	routeGroup := deps.Echo.Group("/api/plugins/participation")

	routeGroup.GET(RouteParticipationEvents, func(c echo.Context) error {
		resp, err := getEvents(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteParticipationEvents, func(c echo.Context) error {

		resp, err := createEvent(c)
		if err != nil {
			return err
		}

		c.Response().Header().Set(echo.HeaderLocation, resp.eventID)
		return restapi.JSONResponse(c, http.StatusCreated, resp)
	})

	routeGroup.GET(RouteParticipationEvent, func(c echo.Context) error {
		resp, err := getEvent(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.DELETE(RouteParticipationEvent, func(c echo.Context) error {
		if err := deleteEvent(c); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})

	routeGroup.GET(RouteParticipationEventStatus, func(c echo.Context) error {
		resp, err := getEventStatus(c)
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

	if err := Plugin.Node.Daemon().BackgroundWorker("Close Participation database", func(ctx context.Context) {
		<-ctx.Done()

		Plugin.LogInfo("Syncing Participation database to disk...")
		if err := deps.ParticipationManager.CloseDatabase(); err != nil {
			Plugin.Panicf("Syncing Participation database to disk... failed: %s", err)
		}
		Plugin.LogInfo("Syncing Participation database to disk... done")
	}, shutdown.PriorityCloseDatabase); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}

	configureEvents()
}

func run() {
	// create a background worker that handles the participation events
	if err := Plugin.Daemon().BackgroundWorker("Participation", func(ctx context.Context) {
		Plugin.LogInfo("Starting Participation ... done")
		attachEvents()
		<-ctx.Done()
		detachEvents()
		Plugin.LogInfo("Stopping Participation ... done")
	}, shutdown.PriorityReferendum); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}

func configureEvents() {

	onUTXOOutput = events.NewClosure(func(index milestone.Index, output *utxo.Output) {
		if err := deps.ParticipationManager.ApplyNewUTXO(index, output); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("participation plugin hit a critical error while applying new UTXO: %s", err.Error()))
		}
	})

	onUTXOSpent = events.NewClosure(func(index milestone.Index, spent *utxo.Spent) {
		if err := deps.ParticipationManager.ApplySpentUTXO(index, spent); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("participation plugin hit a critical error while applying spent TXO: %s", err.Error()))
		}
	})

	onConfirmedMilestoneIndexChanged = events.NewClosure(func(index milestone.Index) {
		if err := deps.ParticipationManager.ApplyNewConfirmedMilestoneIndex(index); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("participation plugin hit a critical error while applying new confirmed milestone index: %s", err.Error()))
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
