package restapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
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
	Plugin             *node.Plugin
	deps               dependencies
	nodeAPIHealthRoute = "/health"

	jwtAuth *jwt.JWTAuth
)

type dependencies struct {
	dig.In
	NodeConfig            *configuration.Configuration `name:"nodeConfig"`
	Tangle                *tangle.Tangle               `optional:"true"`
	Echo                  *echo.Echo
	RestAPIMetrics        *metrics.RestAPIMetrics
	Host                  host.Host
	RestAPIBindAddress    string         `name:"restAPIBindAddress"`
	NodePrivateKey        crypto.PrivKey `name:"nodePrivateKey"`
	DashboardAuthUsername string         `name:"dashboardAuthUsername" optional:"true"`
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
		Plugin.Panic(err)
	}
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *metrics.RestAPIMetrics {
		return &metrics.RestAPIMetrics{}
	}); err != nil {
		Plugin.Panic(err)
	}

	type echoDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
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

		return echoResult{
			Echo:                     e,
			DashboardAllowedAPIRoute: dashboardAllowedAPIRoute,
			FaucetAllowedAPIRoute:    faucetAllowedAPIRoute,
		}
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	// load whitelisted networks
	var whitelistedNetworks []*net.IPNet
	for _, entry := range deps.NodeConfig.Strings(CfgRestAPIWhitelistedAddresses) {
		ipNet, err := utils.ParseIPNet(entry)
		if err != nil {
			Plugin.LogWarnf("Invalid whitelist address: %s", entry)
			continue
		}
		whitelistedNetworks = append(whitelistedNetworks, ipNet)
	}

	permittedRoutes := make(map[string]struct{})
	// load allowed remote access to specific HTTP REST routes
	for _, route := range deps.NodeConfig.Strings(CfgRestAPIPermittedRoutes) {
		permittedRoutes[strings.ToLower(route)] = struct{}{}
	}

	deps.Echo.Use(middlewareFilterRoutes(whitelistedNetworks, permittedRoutes))

	// set basic auth if enabled
	if deps.NodeConfig.Bool(CfgRestAPIJWTAuthEnabled) {

		salt := deps.NodeConfig.String(CfgRestAPIJWTAuthSalt)
		if len(salt) == 0 {
			Plugin.LogFatalf("'%s' should not be empty", CfgRestAPIJWTAuthSalt)
		}

		// API tokens do not expire.
		var err error
		jwtAuth, err = jwt.NewJWTAuth(salt,
			0,
			deps.Host.ID().String(),
			deps.NodePrivateKey,
		)
		if err != nil {
			Plugin.Panicf("JWT auth initialization failed: %w", err)
		}

		excludedRoutes := make(map[string]struct{})
		if deps.NodeConfig.Bool(CfgRestAPIExcludeHealthCheckFromAuth) {
			excludedRoutes[nodeAPIHealthRoute] = struct{}{}
		}

		skipper := func(c echo.Context) bool {
			// check if the route is excluded from basic auth.
			if _, excluded := excludedRoutes[strings.ToLower(c.Path())]; excluded {
				return true
			}
			return false
		}

		allow := func(c echo.Context, subject string, claims *jwt.AuthClaims) bool {
			// Allow all JWT created for the API
			if claims.API {
				return claims.VerifySubject(subject)
			}

			// Only allow Dashboard JWT for certain routes
			if claims.Dashboard {
				if deps.DashboardAuthUsername == "" {
					return false
				}
				return claims.VerifySubject(deps.DashboardAuthUsername) && dashboardAllowedAPIRoute(c)
			}

			return false
		}

		deps.Echo.Use(jwtAuth.Middleware(skipper, allow))
	}

	setupRoutes()
}

func run() {

	Plugin.LogInfo("Starting REST-API server ...")

	if err := Plugin.Daemon().BackgroundWorker("REST-API server", func(shutdownSignal <-chan struct{}) {
		Plugin.LogInfo("Starting REST-API server ... done")

		bindAddr := deps.RestAPIBindAddress
		server := &http.Server{Addr: bindAddr, Handler: deps.Echo}

		go func() {
			Plugin.LogInfof("You can now access the API using: http://%s", bindAddr)
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Plugin.LogWarnf("Stopped REST-API server due to an error (%s)", err)
			}
		}()

		<-shutdownSignal
		Plugin.LogInfo("Stopping REST-API server ...")

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Shutdown(ctx); err != nil {
				Plugin.LogWarn(err)
			}
			cancel()
		}
		Plugin.LogInfo("Stopping REST-API server ... done")
	}, shutdown.PriorityRestAPI); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}

func setupRoutes() {

	deps.Echo.HTTPErrorHandler = func(err error, c echo.Context) {
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

	setupHealthRoute()
}

var dashboardAllowedRoutes = map[string][]string{
	http.MethodGet: {
		"/api/v1/addresses",
		"/api/v1/info",
		"/api/v1/messages",
		"/api/v1/milestones",
		"/api/v1/outputs",
		"/api/v1/peers",
		"/api/v1/transactions",
		"/api/plugins/spammer",
	},
	http.MethodPost: {
		"/api/v1/peers",
		"/api/plugins/spammer",
	},
	http.MethodDelete: {
		"/api/v1/peers",
	},
}

var faucetAllowedRoutes = map[string][]string{
	http.MethodGet: {
		"/api/plugins/faucet/info",
	},
	http.MethodPost: {
		"/api/plugins/faucet/enqueue",
	},
}

func checkAllowedAPIRoute(context echo.Context, allowedRoutes map[string][]string) bool {

	// Check for which route we will allow to access the API
	routesForMethod, exists := allowedRoutes[context.Request().Method]
	if !exists {
		return false
	}

	path := context.Request().URL.EscapedPath()
	for _, prefix := range routesForMethod {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

func dashboardAllowedAPIRoute(context echo.Context) bool {
	return checkAllowedAPIRoute(context, dashboardAllowedRoutes)
}

func faucetAllowedAPIRoute(context echo.Context) bool {
	return checkAllowedAPIRoute(context, faucetAllowedRoutes)
}
