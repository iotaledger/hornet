package restapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/jwt"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/node"
	"github.com/iotaledger/hornet/pkg/restapi"
	"github.com/iotaledger/hornet/pkg/shutdown"
	"github.com/iotaledger/hornet/pkg/tangle"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:           "RestAPI",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Configure:      configure,
			Run:            run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	jwtAuth *jwt.JWTAuth
)

type dependencies struct {
	dig.In
	NodeConfig            *configuration.Configuration `name:"nodeConfig"`
	Tangle                *tangle.Tangle               `optional:"true"`
	Echo                  *echo.Echo
	Host                  host.Host
	RestAPIBindAddress    string         `name:"restAPIBindAddress"`
	NodePrivateKey        crypto.PrivKey `name:"nodePrivateKey"`
	DashboardAuthUsername string         `name:"dashboardAuthUsername" optional:"true"`
	RestRouteManager      *RestRouteManager
}

func initConfigPars(c *dig.Container) {

	type cfgDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type cfgResult struct {
		dig.Out
		RestAPIBindAddress      string `name:"restAPIBindAddress"`
		RestAPILimitsMaxResults int    `name:"restAPILimitsMaxResults"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {
		return cfgResult{
			RestAPIBindAddress:      deps.NodeConfig.String(CfgRestAPIBindAddress),
			RestAPILimitsMaxResults: deps.NodeConfig.Int(CfgRestAPILimitsMaxResults),
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *metrics.RestAPIMetrics {
		return &metrics.RestAPIMetrics{
			Events: &metrics.RestAPIEvents{
				PoWCompleted: events.NewEvent(metrics.PoWCompletedCaller),
			},
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	type echoDeps struct {
		dig.In
		NodeConfig     *configuration.Configuration `name:"nodeConfig"`
		RestAPIMetrics *metrics.RestAPIMetrics
	}

	type echoResult struct {
		dig.Out
		Echo                     *echo.Echo
		DashboardAllowedAPIRoute restapi.AllowedRoute `name:"dashboardAllowedAPIRoute"`
		FaucetAllowedAPIRoute    restapi.AllowedRoute `name:"faucetAllowedAPIRoute"`
	}

	if err := c.Provide(func(deps echoDeps) echoResult {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())
		e.Use(middleware.CORS())
		e.Use(middleware.Gzip())
		e.Use(middleware.BodyLimit(deps.NodeConfig.String(CfgRestAPILimitsMaxBodyLength)))

		e.HTTPErrorHandler = func(err error, c echo.Context) {
			Plugin.LogDebugf("HTTP request failed: %s", err)
			deps.RestAPIMetrics.HTTPRequestErrorCounter.Inc()

			var statusCode int
			var message string

			var e *echo.HTTPError
			if errors.As(err, &e) {
				statusCode = e.Code
				message = fmt.Sprintf("%s, error: %s", e.Message, err)
			} else {
				statusCode = http.StatusInternalServerError
				message = fmt.Sprintf("internal server error. error: %s", err)
			}

			_ = c.JSON(statusCode, restapi.HTTPErrorResponseEnvelope{Error: restapi.HTTPErrorResponse{Code: strconv.Itoa(statusCode), Message: message}})
		}

		return echoResult{
			Echo:                     e,
			DashboardAllowedAPIRoute: dashboardAllowedAPIRoute,
			FaucetAllowedAPIRoute:    faucetAllowedAPIRoute,
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	type proxyDeps struct {
		dig.In
		Echo *echo.Echo
	}

	if err := c.Provide(func(deps proxyDeps) *RestRouteManager {
		return newRestRouteManager(deps.Echo)
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	deps.Echo.Use(apiMiddleware())
	setupRoutes()
}

func run() {

	Plugin.LogInfo("Starting REST-API server ...")

	if err := Plugin.Daemon().BackgroundWorker("REST-API server", func(ctx context.Context) {
		Plugin.LogInfo("Starting REST-API server ... done")

		bindAddr := deps.RestAPIBindAddress

		go func() {
			Plugin.LogInfof("You can now access the API using: http://%s", bindAddr)
			if err := deps.Echo.Start(bindAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Plugin.LogWarnf("Stopped REST-API server due to an error (%s)", err)
			}
		}()

		<-ctx.Done()
		Plugin.LogInfo("Stopping REST-API server ...")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		//nolint:contextcheck // false positive
		if err := deps.Echo.Shutdown(shutdownCtx); err != nil {
			Plugin.LogWarn(err)
		}

		Plugin.LogInfo("Stopping REST-API server ... done")
	}, shutdown.PriorityRestAPI); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

}
