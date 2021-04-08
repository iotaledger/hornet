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
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
		Pluggable: node.Pluggable{
			Name:      "RestAPI",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin             *node.Plugin
	log                *logger.Logger
	deps               dependencies
	nodeAPIHealthRoute = "/health"

	jwtAuth *jwt.JWTAuth
)

type dependencies struct {
	dig.In
	NodeConfig     *configuration.Configuration `name:"nodeConfig"`
	Tangle         *tangle.Tangle
	Echo           *echo.Echo
	RestAPIMetrics *metrics.RestAPIMetrics
	Host           host.Host
	NodePrivateKey crypto.PrivKey
}

func provide(c *dig.Container) {

	if err := c.Provide(func() *metrics.RestAPIMetrics {
		return &metrics.RestAPIMetrics{}
	}); err != nil {
		panic(err)
	}

	type echodeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type resultdeps struct {
		dig.Out
		Echo                     *echo.Echo
		DashboardAllowedAPIRoute restapi.AllowedRoute
	}

	if err := c.Provide(func(deps echodeps) resultdeps {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())
		e.Use(middleware.CORS())
		e.Use(middleware.Gzip())
		e.Use(middleware.BodyLimit(deps.NodeConfig.String(CfgRestAPILimitsMaxBodyLength)))
		return resultdeps{
			Echo:                     e,
			DashboardAllowedAPIRoute: dashboardAllowedAPIRoute,
		}
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	// load whitelisted networks
	var whitelistedNetworks []*net.IPNet
	for _, entry := range deps.NodeConfig.Strings(CfgRestAPIWhitelistedAddresses) {
		ipNet, err := utils.ParseIPNet(entry)
		if err != nil {
			log.Warnf("Invalid whitelist address: %s", entry)
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
	if deps.NodeConfig.Bool(CfgRestAPIAuthEnabled) {

		salt := deps.NodeConfig.String(CfgRestAPIAuthSalt)
		if len(salt) == 0 {
			log.Fatalf("'%s' should not be empty", CfgRestAPIAuthSalt)
		}

		// API tokens do not expire.
		jwtAuth = jwt.NewJWTAuth(salt,
			0,
			deps.Host.ID().String(),
			deps.NodePrivateKey,
		)

		t, err := jwtAuth.IssueJWT(true, false)
		if err != nil {
			panic(err)
		}
		log.Infof("You can use the following JWT to access the API: %s", t)

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

		allow := func(c echo.Context, claims *jwt.AuthClaims) bool {
			// Allow all JWT created for the API
			if claims.API {
				return true
			}

			// Only allow Dashboard JWT for certain routes
			if claims.Dashboard {
				return dashboardAllowedAPIRoute(c)
			}

			return false
		}

		deps.Echo.Use(jwtAuth.Middleware(skipper, allow))
	}

	setupRoutes()
}

func run() {
	log.Info("Starting REST-API server ...")

	Plugin.Daemon().BackgroundWorker("REST-API server", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting REST-API server ... done")

		bindAddr := deps.NodeConfig.String(CfgRestAPIBindAddress)
		server := &http.Server{Addr: bindAddr, Handler: deps.Echo}

		go func() {
			log.Infof("You can now access the API using: http://%s", bindAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Warnf("Stopped REST-API server due to an error (%s)", err)
			}
		}()

		<-shutdownSignal
		log.Info("Stopping REST-API server ...")

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Shutdown(ctx); err != nil {
				log.Warn(err.Error())
			}
			cancel()
		}
		log.Info("Stopping REST-API server ... done")
	}, shutdown.PriorityRestAPI)
}

func setupRoutes() {

	deps.Echo.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Debugf("HTTP request failed: %s", err)
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

		c.JSON(statusCode, restapi.HTTPErrorResponseEnvelope{Error: restapi.HTTPErrorResponse{Code: strconv.Itoa(statusCode), Message: message}})
	}

	setupHealthRoute()
}

var allowedRoutes = map[string][]string{
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

func dashboardAllowedAPIRoute(context echo.Context) bool {

	// Check for which route we will allow the dashboard to access the API
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
