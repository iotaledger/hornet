package restapi

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusEnabled,
		Component: &app.Component{
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
	Plugin             *app.Plugin
	deps               dependencies
	nodeAPIHealthRoute = "/health"

	jwtAuth *jwt.JWTAuth
)

type dependencies struct {
	dig.In
	AppConfig             *configuration.Configuration `name:"appConfig"`
	Tangle                *tangle.Tangle               `optional:"true"`
	Echo                  *echo.Echo
	RestAPIMetrics        *metrics.RestAPIMetrics
	Host                  host.Host
	RestAPIBindAddress    string         `name:"restAPIBindAddress"`
	NodePrivateKey        crypto.PrivKey `name:"nodePrivateKey"`
	DashboardAuthUsername string         `name:"dashboardAuthUsername" optional:"true"`
}

func initConfigPars(c *dig.Container) error {

	type cfgDeps struct {
		dig.In
		AppConfig *configuration.Configuration `name:"appConfig"`
	}

	type cfgResult struct {
		dig.Out
		RestAPIBindAddress      string `name:"restAPIBindAddress"`
		RestAPILimitsMaxResults int    `name:"restAPILimitsMaxResults"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {
		return cfgResult{
			RestAPIBindAddress:      deps.AppConfig.String(CfgRestAPIBindAddress),
			RestAPILimitsMaxResults: deps.AppConfig.Int(CfgRestAPILimitsMaxResults),
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func provide(c *dig.Container) error {

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
		AppConfig *configuration.Configuration `name:"appConfig"`
	}

	type echoResult struct {
		dig.Out
		Echo                     *echo.Echo
		DashboardAllowedAPIRoute restapi.AllowedRoute `name:"dashboardAllowedAPIRoute"`
	}

	if err := c.Provide(func(deps echoDeps) echoResult {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())
		e.Use(middleware.CORS())
		e.Use(middleware.Gzip())
		e.Use(middleware.BodyLimit(deps.AppConfig.String(CfgRestAPILimitsMaxBodyLength)))

		return echoResult{
			Echo:                     e,
			DashboardAllowedAPIRoute: dashboardAllowedAPIRoute,
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	type proxyDeps struct {
		dig.In
		Echo *echo.Echo
	}

	if err := c.Provide(func(deps proxyDeps) *RestPluginManager {
		return newRestPluginManager(deps.Echo)
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {
	deps.Echo.Use(apiMiddleware())
	setupRoutes()

	return nil
}

func run() error {

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
		if err := deps.Echo.Shutdown(shutdownCtx); err != nil {
			Plugin.LogWarn(err)
		}
		shutdownCtxCancel()
		Plugin.LogInfo("Stopping REST-API server ... done")
	}, shutdown.PriorityRestAPI); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func setupRoutes() {

	errorHandler := restapi.ErrorHandler()

	deps.Echo.HTTPErrorHandler = func(err error, c echo.Context) {
		Plugin.LogDebugf("HTTP request failed: %s", err)
		deps.RestAPIMetrics.HTTPRequestErrorCounter.Inc()

		errorHandler(err, c)
	}

	setupHealthRoute()
}
