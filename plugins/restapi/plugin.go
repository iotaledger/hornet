package restapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/basicauth"
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
)

type dependencies struct {
	dig.In
	NodeConfig     *configuration.Configuration `name:"nodeConfig"`
	Tangle         *tangle.Tangle
	Echo           *echo.Echo
	RestAPIMetrics *metrics.RestAPIMetrics
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
	if err := c.Provide(func(deps echodeps) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())
		e.Use(middleware.CORS())
		e.Use(middleware.Gzip())
		e.Use(middleware.BodyLimit(deps.NodeConfig.String(CfgRestAPILimitsMaxBodyLength)))
		return e
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
	if deps.NodeConfig.Bool(CfgRestAPIBasicAuthEnabled) {
		// grab auth info
		expectedUsername := deps.NodeConfig.String(CfgRestAPIBasicAuthUsername)
		if len(expectedUsername) == 0 {
			log.Fatalf("'%s' must not be empty if web API basic auth is enabled", CfgRestAPIBasicAuthUsername)
		}

		expectedPasswordHashHex := deps.NodeConfig.String(CfgRestAPIBasicAuthPasswordHash)
		if len(expectedPasswordHashHex) != 64 {
			log.Fatalf("'%s' must be 64 (hex encoded scrypt hash) in length if web API basic auth is enabled", CfgRestAPIBasicAuthPasswordHash)
		}

		expectedPasswordHash, err := hex.DecodeString(expectedPasswordHashHex)
		if err != nil {
			log.Fatalf("'%s' must be hex encoded", CfgRestAPIBasicAuthPasswordHash)
		}

		passwordSaltHex := deps.NodeConfig.String(CfgRestAPIBasicAuthPasswordSalt)
		passwordSalt, err := hex.DecodeString(passwordSaltHex)
		if err != nil {
			log.Fatalf("'%s' must be hex encoded", CfgRestAPIBasicAuthPasswordSalt)
		}

		excludedRoutes := make(map[string]struct{})
		if deps.NodeConfig.Bool(CfgRestAPIExcludeHealthCheckFromAuth) {
			excludedRoutes[nodeAPIHealthRoute] = struct{}{}
		}

		deps.Echo.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
			Skipper: func(c echo.Context) bool {
				// check if the route is excluded from basic auth.
				if _, excluded := excludedRoutes[strings.ToLower(c.Path())]; excluded {
					return true
				}
				return false
			},
			Validator: func(username, password string, c echo.Context) (bool, error) {
				if username != expectedUsername {
					return false, nil
				}

				if valid, _ := basicauth.VerifyPassword([]byte(password), []byte(passwordSalt), []byte(expectedPasswordHash)); !valid {
					return false, nil
				}

				return true, nil
			},
		}))
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

		switch errors.Cause(err) {

		case echo.ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case echo.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
			message = "unauthorized"

		case restapi.ErrForbidden:
			statusCode = http.StatusForbidden
			message = "access forbidden"

		case restapi.ErrServiceUnavailable:
			statusCode = http.StatusServiceUnavailable
			message = "service unavailable"

		case restapi.ErrServiceNotImplemented:
			statusCode = http.StatusNotImplemented
			message = "service not implemented"

		case restapi.ErrInternalError:
			statusCode = http.StatusInternalServerError
			message = "internal server error"

		case restapi.ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case restapi.ErrInvalidParameter:
			statusCode = http.StatusBadRequest
			message = "bad request"

		default:
			statusCode = http.StatusInternalServerError
			message = "internal server error"
		}

		message = fmt.Sprintf("%s, error: %s", message, err)

		c.JSON(statusCode, restapi.HTTPErrorResponseEnvelope{Error: restapi.HTTPErrorResponse{Code: strconv.Itoa(statusCode), Message: message}})
	}

	setupHealthRoute()
}
