package webapi

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/bytes"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/model/tangle"
	"github.com/iotaledger/hornet/pkg/shutdown"
)

const (
	waitForNodeSyncedTimeout = 2000 * time.Millisecond
)

// PLUGIN WebAPI
var (
	PLUGIN = node.NewPlugin("WebAPI", node.Enabled, configure, run)
	log    *logger.Logger

	e            *echo.Echo
	webAPIServer *WebAPIServer
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	e = echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.Gzip())
	e.Use(middleware.BodyLimit(bytes.Format(int64(config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxBodyLengthBytes)))))
	e.Use(apiMiddleware())

	healthzRoute()

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		webAPIServer = NewWebAPIServer(
			e,
			log,
			config.NodeConfig.GetStringSlice(config.CfgWebAPIPublicRPCEndpoints),
			config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxResults))
	}
}

func run(_ *node.Plugin) {
	log.Info("Starting WebAPI server ...")

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		features := make([]string, 0)

		// Check for features
		if ok := webAPIServer.HasRPCEndpoint("attachtotangle"); ok {
			features = append(features, "RemotePOW")
		}

		if tangle.GetSnapshotInfo().IsSpentAddressesEnabled() {
			features = append(features, "WereAddressesSpentFrom")
		}

		webAPIServer.SetFeatures(features)
	}

	daemon.BackgroundWorker("WebAPI server", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting WebAPI server ... done")

		bindAddr := config.NodeConfig.GetString(config.CfgWebAPIBindAddress)

		go func() {
			log.Infof("You can now access the API using: http://%s", bindAddr)
			if err := e.Start(bindAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Warn("Stopped WebAPI server due to an error ... done")
			}
		}()

		<-shutdownSignal
		log.Info("Stopping WebAPI server ...")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		//nolint:contextcheck // false positive
		if err := e.Shutdown(shutdownCtx); err != nil {
			log.Warn(err)
		}

		log.Info("Stopping WebAPI server ... done")
	}, shutdown.PriorityAPI)
}
