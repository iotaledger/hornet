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
	restapiv1 "github.com/gohornet/hornet/plugins/restapi/v1"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// ParameterParticipationEventID is used to identify an event by its ID.
	ParameterParticipationEventID = "eventID"
)

const (

	// RouteParticipationEvents is the route to list all events, returning their ID, the event name and status.
	// GET returns a list of all events known to the node. Optional query parameter returns filters events by type (query parameters: "type").
	RouteParticipationEvents = "/events"

	// RouteParticipationEvent is the route to access a single participation by its ID.
	// GET gives a quick overview of the participation. This does not include the current standings.
	RouteParticipationEvent = "/events/:" + ParameterParticipationEventID

	// RouteParticipationEventStatus is the route to access the status of a single participation by its ID.
	// GET returns the amount of tokens participating and accumulated votes for the ballot if the event contains a ballot. Optional query parameter returns the status for the given milestone index (query parameters: "milestoneIndex").
	RouteParticipationEventStatus = "/events/:" + ParameterParticipationEventID + "/status"

	// RouteOutputStatus is the route to get the vote status for a given outputID.
	// GET returns the messageID the participation was included, the starting and ending milestone index this participation was tracked.
	RouteOutputStatus = "/outputs/:" + restapi.ParameterOutputID

	// RouteAddressBech32Status is the route to get the staking rewards for the given bech32 address.
	RouteAddressBech32Status = "/addresses/:" + restapi.ParameterAddress

	// RouteAddressBech32Outputs is the route to get the outputs for the given bech32 address.
	RouteAddressBech32Outputs = "/addresses/:" + restapi.ParameterAddress + "/outputs"

	// RouteAddressEd25519Status is the route to get the staking rewards for the given ed25519 address.
	RouteAddressEd25519Status = "/addresses/ed25519/:" + restapi.ParameterAddress

	// RouteAddressEd25519Outputs is the route to get the outputs for the given ed25519 address.
	RouteAddressEd25519Outputs = "/addresses/ed25519/:" + restapi.ParameterAddress + "/outputs"

	// RouteAdminCreateEvent is the route the node operator can use to add events.
	// POST creates a new event to track
	RouteAdminCreateEvent = "/admin/events"

	// RouteAdminDeleteEvent is the route the node operator can use to remove events.
	// DELETE removes a tracked participation.
	RouteAdminDeleteEvent = "/admin/events/:" + ParameterParticipationEventID

	// RouteAdminActiveParticipations is the route the node operator can use to get all the active participations for a certain event.
	// GET returns a list of all active participations
	RouteAdminActiveParticipations = "/admin/events/:" + ParameterParticipationEventID + "/active"

	// RouteAdminPastParticipations is the route the node operator can use to get all the past participations for a certain event.
	// GET returns a list of all past participations
	RouteAdminPastParticipations = "/admin/events/:" + ParameterParticipationEventID + "/past"

	// RouteAdminRewards is the route the node operator can use to get the rewards for a staking event.
	// GET retrieves the staking event rewards.
	RouteAdminRewards = "/admin/events/:" + ParameterParticipationEventID + "/rewards"
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

	onLedgerUpdated *events.Closure
)

type dependencies struct {
	dig.In
	NodeConfig           *configuration.Configuration `name:"nodeConfig"`
	ParticipationManager *participation.ParticipationManager
	UTXOManager          *utxo.Manager
	SyncManager          *syncmanager.SyncManager
	Tangle               *tangle.Tangle
	Echo                 *echo.Echo
	Bech32HRP            iotago.NetworkPrefix `name:"bech32HRP"`
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
			Plugin.LogPanic(err)
		}

		pm, err := participation.NewManager(
			deps.Storage,
			deps.SyncManager,
			participationStore,
		)
		if err != nil {
			Plugin.LogPanic(err)
		}
		return pm
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	restapiv1.AddFeature(Plugin.Name)

	routeGroup := deps.Echo.Group("/api/plugins/participation")

	routeGroup.GET(RouteParticipationEvents, func(c echo.Context) error {
		resp, err := getEvents(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteAdminCreateEvent, func(c echo.Context) error {

		resp, err := createEvent(c)
		if err != nil {
			return err
		}

		c.Response().Header().Set(echo.HeaderLocation, resp.EventID)
		return restapi.JSONResponse(c, http.StatusCreated, resp)
	})

	routeGroup.GET(RouteParticipationEvent, func(c echo.Context) error {
		resp, err := getEvent(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.DELETE(RouteAdminDeleteEvent, func(c echo.Context) error {
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

	routeGroup.GET(RouteAddressBech32Status, func(c echo.Context) error {
		resp, err := getRewardsByBech32Address(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressBech32Outputs, func(c echo.Context) error {
		resp, err := getOutputsByBech32Address(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressEd25519Status, func(c echo.Context) error {
		resp, err := getRewardsByEd25519Address(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAddressEd25519Outputs, func(c echo.Context) error {
		resp, err := getOutputsByEd25519Address(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAdminActiveParticipations, func(c echo.Context) error {
		resp, err := getActiveParticipations(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAdminPastParticipations, func(c echo.Context) error {
		resp, err := getPastParticipations(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteAdminRewards, func(c echo.Context) error {
		resp, err := getRewards(c)
		if err != nil {
			return err
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	if err := Plugin.Node.Daemon().BackgroundWorker("Close Participation database", func(ctx context.Context) {
		<-ctx.Done()

		Plugin.LogInfo("Syncing Participation database to disk...")
		if err := deps.ParticipationManager.CloseDatabase(); err != nil {
			Plugin.LogPanicf("Syncing Participation database to disk... failed: %s", err)
		}
		Plugin.LogInfo("Syncing Participation database to disk... done")
	}, shutdown.PriorityCloseDatabase); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
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
	}, shutdown.PriorityParticipation); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}

func configureEvents() {

	onLedgerUpdated = events.NewClosure(func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
		if err := deps.ParticipationManager.ApplyNewUTXOs(index, newOutputs); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("participation plugin hit a critical error while applying new UTXO: %s", err.Error()))
		}
		if err := deps.ParticipationManager.ApplySpentUTXOs(index, newSpents); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("participation plugin hit a critical error while applying spent TXO: %s", err.Error()))
		}
		if err := deps.ParticipationManager.ApplyNewConfirmedMilestoneIndex(index); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("participation plugin hit a critical error while applying new confirmed milestone index: %s", err.Error()))
		}
	})
}

func attachEvents() {
	deps.Tangle.Events.LedgerUpdated.Attach(onLedgerUpdated)
}

func detachEvents() {
	deps.Tangle.Events.LedgerUpdated.Detach(onLedgerUpdated)
}
