package restapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	cnet "github.com/projectcalico/libcalico-go/lib/net"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/pkg/basicauth"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
)

func init() {
	Plugin = &node.Plugin{
		Name:      "RestAPI",
		DepsFunc:  func(cDeps dependencies) { deps = cDeps },
		Provide:   provide,
		Configure: configure,
		Run:       run,
		Status:    node.Enabled,
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
	NodeConfig *configuration.Configuration `name:"nodeConfig"`
	Echo       *echo.Echo
}

func provide(c *dig.Container) {
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
		e.Use(middleware.BodyLimit(deps.NodeConfig.String(config.CfgRestAPILimitsMaxBodyLength)))
		return e
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	// load whitelisted networks
	var whitelistedNetworks []net.IPNet
	for _, entry := range deps.NodeConfig.Strings(config.CfgRestAPIWhitelistedAddresses) {
		_, ipnet, err := cnet.ParseCIDROrIP(entry)
		if err != nil {
			log.Warnf("Invalid whitelist address: %s", entry)
			continue
		}
		whitelistedNetworks = append(whitelistedNetworks, ipnet.IPNet)
	}

	permittedRoutes := make(map[string]struct{})
	// load allowed remote access to specific HTTP REST routes
	for _, route := range deps.NodeConfig.Strings(config.CfgRestAPIPermittedRoutes) {
		permittedRoutes[strings.ToLower(route)] = struct{}{}
	}

	deps.Echo.Use(middlewareFilterRoutes(whitelistedNetworks, permittedRoutes))

	exclHealthCheckFromAuth := deps.NodeConfig.Bool(config.CfgRestAPIExcludeHealthCheckFromAuth)
	if exclHealthCheckFromAuth {
		// Handle route without auth
		setupHealthRoute()
	}

	// set basic auth if enabled
	if deps.NodeConfig.Bool(config.CfgRestAPIBasicAuthEnabled) {
		// grab auth info
		expectedUsername := deps.NodeConfig.String(config.CfgRestAPIBasicAuthUsername)
		expectedPasswordHash := deps.NodeConfig.String(config.CfgRestAPIBasicAuthPasswordHash)
		passwordSalt := deps.NodeConfig.String(config.CfgRestAPIBasicAuthPasswordSalt)

		if len(expectedUsername) == 0 {
			log.Fatalf("'%s' must not be empty if web API basic auth is enabled", config.CfgRestAPIBasicAuthUsername)
		}

		if len(expectedPasswordHash) != 64 {
			log.Fatalf("'%s' must be 64 (sha256 hash) in length if web API basic auth is enabled", config.CfgRestAPIBasicAuthPasswordHash)
		}

		deps.Echo.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username != expectedUsername {
				return false, nil
			}

			if valid, _ := basicauth.VerifyPassword([]byte(password), []byte(passwordSalt), []byte(expectedPasswordHash)); !valid {
				return false, nil
			}

			return true, nil
		}))
	}

	setupRoutes(exclHealthCheckFromAuth)
}

func run() {
	log.Info("Starting REST-API server ...")

	Plugin.Daemon().BackgroundWorker("REST-API server", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting REST-API server ... done")

		bindAddr := deps.NodeConfig.String(config.CfgRestAPIBindAddress)
		server := &http.Server{Addr: bindAddr, Handler: deps.Echo}

		go func() {
			log.Infof("You can now access the API using: http://%s", bindAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Warnf("Stopped REST-API server due to an error (%w)", err)
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

func setupRoutes(exclHealthCheckFromAuth bool) {

	deps.Echo.HTTPErrorHandler = func(err error, c echo.Context) {
		c.Logger().Error(err)

		var statusCode int
		var message string

		switch errors.Cause(err) {

		case echo.ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case echo.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
			message = "unauthorized"

		case common.ErrForbidden:
			statusCode = http.StatusForbidden
			message = "access forbidden"

		case common.ErrServiceUnavailable:
			statusCode = http.StatusServiceUnavailable
			message = "service unavailable"

		case common.ErrInternalError:
			statusCode = http.StatusInternalServerError
			message = "internal server error"

		case common.ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case common.ErrInvalidParameter:
			statusCode = http.StatusBadRequest
			message = "bad request"

		default:
			statusCode = http.StatusInternalServerError
			message = "internal server error"
		}

		message = fmt.Sprintf("%s, error: %+v", message, err)

		c.JSON(statusCode, common.HTTPErrorResponseEnvelope{Error: common.HTTPErrorResponse{Code: string(statusCode), Message: message}})
	}

	if !exclHealthCheckFromAuth {
		// Handle route with auth
		setupHealthRoute()
	}
}
