package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	coreDatabase "github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/migrator"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/v2/pkg/pruning"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/pkg/tipselect"
	"github.com/iotaledger/hornet/v2/plugins/inx"
)

// routeMetrics is the route for getting the prometheus metrics.
// GET returns metrics.
const (
	routeMetrics = "/metrics"
)

func init() {
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:      "Prometheus",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
		IsEnabled: func() bool {
			return ParamsPrometheus.Enabled
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies

	registry = prometheus.NewRegistry()
	collects []func()
)

type dependencies struct {
	dig.In
	AppInfo          *app.Info
	SyncManager      *syncmanager.SyncManager
	ServerMetrics    *metrics.ServerMetrics
	Storage          *storage.Storage
	StorageMetrics   *metrics.StorageMetrics
	TangleDatabase   *database.Database      `name:"tangleDatabase"`
	UTXODatabase     *database.Database      `name:"utxoDatabase"`
	RestAPIMetrics   *metrics.RestAPIMetrics `optional:"true"`
	INXMetrics       *metrics.INXMetrics     `optional:"true"`
	GossipService    *gossip.Service
	ReceiptService   *migrator.ReceiptService `optional:"true"`
	Tangle           *tangle.Tangle
	PeeringManager   *p2p.Manager
	RequestQueue     gossip.RequestQueue
	MessageProcessor *gossip.MessageProcessor
	TipSelector      *tipselect.TipSelector `optional:"true"`
	SnapshotManager  *snapshot.Manager
	PruningManager   *pruning.Manager
	Echo             *echo.Echo  `optional:"true"`
	PrometheusEcho   *echo.Echo  `name:"prometheusEcho"`
	INXServer        *inx.Server `optional:"true"`
}

func provide(c *dig.Container) error {

	type depsOut struct {
		dig.Out
		PrometheusEcho *echo.Echo `name:"prometheusEcho"`
	}

	if err := c.Provide(func() depsOut {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())

		return depsOut{
			PrometheusEcho: e,
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {
	if ParamsPrometheus.DatabaseMetrics {
		configureDatabase(coreDatabase.TangleDatabaseDirectoryName, deps.TangleDatabase)
		configureDatabase(coreDatabase.UTXODatabaseDirectoryName, deps.UTXODatabase)
		configureStorage(deps.Storage, deps.StorageMetrics)
	}
	if ParamsPrometheus.NodeMetrics {
		configureNode()
	}
	if ParamsPrometheus.GossipMetrics {
		configureGossipPeers()
		configureGossipNode()
	}
	if ParamsPrometheus.CachesMetrics {
		configureCaches()
	}
	if ParamsPrometheus.RestAPIMetrics && deps.RestAPIMetrics != nil {
		configureRestAPI()
	}
	if ParamsPrometheus.INXMetrics && deps.INXMetrics != nil {
		configureINX()
	}
	if ParamsPrometheus.INXMetrics && deps.INXServer != nil {
		deps.INXServer.ConfigurePrometheus()
		registry.MustRegister(grpcprometheus.DefaultServerMetrics)
	}
	if ParamsPrometheus.MigrationMetrics {
		if deps.ReceiptService != nil {
			configureReceipts()
		}
	}
	if ParamsPrometheus.DebugMetrics {
		configureDebug()
	}
	if ParamsPrometheus.GoMetrics {
		registry.MustRegister(collectors.NewGoCollector())
	}
	if ParamsPrometheus.ProcessMetrics {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	return nil
}

func addCollect(collect func()) {
	collects = append(collects, collect)
}

type fileservicediscovery struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func writeFileServiceDiscoveryFile() {
	path := ParamsPrometheus.FileServiceDiscovery.Path
	d := []fileservicediscovery{{
		Targets: []string{ParamsPrometheus.FileServiceDiscovery.Target},
		Labels:  make(map[string]string),
	}}
	j, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		Plugin.LogPanic("unable to marshal file service discovery JSON:", err)

		return
	}

	// this truncates an existing file
	//nolint:gosec // users should be able to read the file
	if err := os.WriteFile(path, j, 0o640); err != nil {
		Plugin.LogPanic("unable to write file service discovery file:", err)
	}

	Plugin.LogInfof("Wrote 'file service discovery' content to %s", path)
}

func run() error {
	Plugin.LogInfo("Starting Prometheus exporter ...")

	if ParamsPrometheus.FileServiceDiscovery.Enabled {
		writeFileServiceDiscoveryFile()
	}

	if err := Plugin.Daemon().BackgroundWorker("Prometheus exporter", func(ctx context.Context) {
		Plugin.LogInfo("Starting Prometheus exporter ... done")

		deps.PrometheusEcho.GET(routeMetrics, func(c echo.Context) error {
			for _, collect := range collects {
				collect()
			}

			handler := promhttp.HandlerFor(
				registry,
				promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				},
			)

			if ParamsPrometheus.PromhttpMetrics {
				handler = promhttp.InstrumentMetricHandler(registry, handler)
			}

			handler.ServeHTTP(c.Response().Writer, c.Request())

			return nil
		})

		bindAddr := ParamsPrometheus.BindAddress

		go func() {
			Plugin.LogInfof("You can now access the Prometheus exporter using: http://%s/metrics", bindAddr)
			if err := deps.PrometheusEcho.Start(bindAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Plugin.LogWarnf("Stopped Prometheus exporter due to an error (%s)", err)
			}
		}()

		<-ctx.Done()
		Plugin.LogInfo("Stopping Prometheus exporter ...")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		//nolint:contextcheck // false positive
		err := deps.PrometheusEcho.Shutdown(shutdownCtx)
		if err != nil {
			Plugin.LogWarn(err)
		}

		Plugin.LogInfo("Stopping Prometheus exporter ... done")
	}, daemon.PriorityPrometheus); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}
