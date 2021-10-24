package referendum

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/referendum"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
)

const (
	// ParameterReferendumID is used to identify a referendum by it's ID.
	ParameterReferendumID = "referendumID"

	// ParameterQuestionIndex is used to identify a question by it's index.
	ParameterQuestionIndex = "questionIndex"
)

const (

	// RouteReferendum is the route to list all referenda, returning their UUID, the referendum name and status.
	// GET returns a list of all referenda known to the node.
	// POST creates a new vote to track
	RouteReferenda = "/referendum"

	// GET gives a quick overview of the referendum. This does not include the questions or current standings.
	// DELETE removes a tracked vote
	RouteReferendum = "/referendum/:" + ParameterReferendumID

	// GET returns the entire vote with all questions, but not current standings.
	RouteReferendumQuestions = "/referendum/:" + ParameterReferendumID + "/questions"

	// GET returns information and vote options for a specific question.
	RouteReferendumQuestion = "/referendum/:" + ParameterReferendumID + "/questions/" + ParameterQuestionIndex

	// GET returns the amount of tokens voting and the weight on each option of every question.
	RouteReferendumStatus = "/referendum/:" + ParameterReferendumID + "/status"

	// GET returns the amount of tokens voting for each option on the specified question.
	RouteReferendumQuestionStatus = "/referendum/:" + ParameterReferendumID + "/status/" + ParameterQuestionIndex
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
	ReferendumManager *referendum.ReferendumManager
	Tangle            *tangle.Tangle
	Echo              *echo.Echo
	ShutdownHandler   *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	type referendumDeps struct {
		dig.In
		Storage    *storage.Storage
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps referendumDeps) *referendum.ReferendumManager {
		return referendum.NewManager(
			deps.Storage,
			referendum.WithLogger(Plugin.Logger()),
		)
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	routeGroup := deps.Echo.Group("/api/plugins/referendum")

	routeGroup.GET(RouteReferenda, func(c echo.Context) error {
		resp, err := getReferenda(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteReferenda, func(c echo.Context) error {
		resp, err := createReferendum(c)
		if err != nil {
			return err
		}

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

	routeGroup.GET(RouteReferendumQuestions, func(c echo.Context) error {
		resp, err := getReferendumQuestions(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteReferendumQuestion, func(c echo.Context) error {
		resp, err := getReferendumQuestion(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteReferendumStatus, func(c echo.Context) error {
		resp, err := getReferendumStatus(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.GET(RouteReferendumQuestionStatus, func(c echo.Context) error {
		resp, err := getReferendumQuestionStatus(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	configureEvents()
}

func run() {
	// create a background worker that handles the referendum events
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
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("referendum plugin hit a critical error while applying new UTXO: %s", err.Error()))
		}
	})

	onUTXOSpent = events.NewClosure(func(index milestone.Index, spent *utxo.Spent) {
		if err := deps.ReferendumManager.ApplySpentUTXO(index, spent); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("referendum plugin hit a critical error while applying spent TXO: %s", err.Error()))
		}
	})

	onConfirmedMilestoneIndexChanged = events.NewClosure(func(index milestone.Index) {
		if err := deps.ReferendumManager.ApplyNewConfirmedMilestoneIndex(index); err != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("referendum plugin hit a critical error while applying new confirmed milestone index: %s", err.Error()))
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
