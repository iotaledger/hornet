package restapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"

	cnet "github.com/projectcalico/libcalico-go/lib/net"

	"github.com/gohornet/hornet/pkg/basicauth"
	"github.com/gohornet/hornet/plugins/restapi/common"
	v1 "github.com/gohornet/hornet/plugins/restapi/v1"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	// The route for the health REST API call.
	NodeAPIHealthRoute = "/health"
)

var (
	PLUGIN = node.NewPlugin("RestAPI", node.Enabled, configure, run)
	log    *logger.Logger

	server               *http.Server
	permittedRoutes      = make(map[string]struct{})
	whitelistedNetworks  []net.IPNet
	e                    *echo.Echo
	serverShutdownSignal <-chan struct{}
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	e = echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.Gzip())
	e.Use(middleware.BodyLimit(config.NodeConfig.GetString(config.CfgRestAPILimitsMaxBodyLength)))

	// Load allowed remote access to specific HTTP REST routes
	cfgPermittedRoutes := config.NodeConfig.GetStringSlice(config.CfgRestAPIPermittedRoutes)
	if len(cfgPermittedRoutes) > 0 {
		for _, route := range cfgPermittedRoutes {
			permittedRoutes[strings.ToLower(route)] = struct{}{}
		}
	}

	// load whitelisted addresses
	whitelist := append([]string{"127.0.0.1", "::1"}, config.NodeConfig.GetStringSlice(config.CfgRestAPIWhitelistedAddresses)...)
	for _, entry := range whitelist {
		_, ipnet, err := cnet.ParseCIDROrIP(entry)
		if err != nil {
			log.Warnf("Invalid whitelist address: %s", entry)
			continue
		}
		whitelistedNetworks = append(whitelistedNetworks, ipnet.IPNet)
	}

	exclHealthCheckFromAuth := config.NodeConfig.GetBool(config.CfgRestAPIExcludeHealthCheckFromAuth)
	if exclHealthCheckFromAuth {
		// Handle route without auth
		setupHealthRoute(e)
	}

	// set basic auth if enabled
	if config.NodeConfig.GetBool(config.CfgRestAPIBasicAuthEnabled) {
		// grab auth info
		expectedUsername := config.NodeConfig.GetString(config.CfgRestAPIBasicAuthUsername)
		expectedPasswordHash := config.NodeConfig.GetString(config.CfgRestAPIBasicAuthPasswordHash)
		passwordSalt := config.NodeConfig.GetString(config.CfgRestAPIBasicAuthPasswordSalt)

		if len(expectedUsername) == 0 {
			log.Fatalf("'%s' must not be empty if web API basic auth is enabled", config.CfgRestAPIBasicAuthUsername)
		}

		if len(expectedPasswordHash) != 64 {
			log.Fatalf("'%s' must be 64 (sha256 hash) in length if web API basic auth is enabled", config.CfgRestAPIBasicAuthPasswordHash)
		}

		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == expectedUsername &&
				basicauth.VerifyPassword(password, passwordSalt, expectedPasswordHash) {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(e, exclHealthCheckFromAuth)
}

func run(_ *node.Plugin) {
	log.Info("Starting REST-API server ...")

	daemon.BackgroundWorker("REST-API server", func(shutdownSignal <-chan struct{}) {
		serverShutdownSignal = shutdownSignal

		log.Info("Starting REST-API server ... done")

		bindAddr := config.NodeConfig.GetString(config.CfgRestAPIBindAddress)
		server = &http.Server{Addr: bindAddr, Handler: e}

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
			err := server.Shutdown(ctx)
			if err != nil {
				log.Warn(err.Error())
			}
			cancel()
		}
		log.Info("Stopping REST-API server ... done")
	}, shutdown.PriorityRestAPI)
}

func setupRoutes(e *echo.Echo, exclHealthCheckFromAuth bool) {

	e.HTTPErrorHandler = func(err error, c echo.Context) {
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

		c.JSON(statusCode, common.HTTPErrorResponseEnvelope{Error: common.HTTPErrorResponse{Code: statusCode, Message: message}})
	}

	if !exclHealthCheckFromAuth {
		// Handle route with auth
		setupHealthRoute(e)
	}

	if config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		// do not setup additional routes if the nodes is an entry node
		return
	}

	v1.SetupApiRoutesV1(e.Group("/api/v1"))
}
