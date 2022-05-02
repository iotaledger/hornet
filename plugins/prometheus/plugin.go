package prometheus

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/inx"
	"github.com/iotaledger/hive.go/configuration"
)

// RouteMetrics is the route for getting the prometheus metrics.
// GET returns metrics.
const (
	RouteMetrics = "/metrics"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Prometheus",
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

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()
)

type dependencies struct {
	dig.In
	AppInfo          *app.AppInfo
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
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
	SnapshotManager  *snapshot.SnapshotManager
	PrometheusEcho   *echo.Echo     `name:"prometheusEcho"`
	INXServer        *inx.INXServer `optional:"true"`
}

func provide(c *dig.Container) {

	type depsIn struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type depsOut struct {
		dig.Out
		PrometheusEcho *echo.Echo `name:"prometheusEcho"`
	}

	if err := c.Provide(func(depsIn) depsOut {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())
		return depsOut{
			PrometheusEcho: e,
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	if deps.NodeConfig.Bool(CfgPrometheusDatabase) {
		configureDatabase(coreDatabase.TangleDatabaseDirectoryName, deps.TangleDatabase)
		configureDatabase(coreDatabase.UTXODatabaseDirectoryName, deps.UTXODatabase)
		configureStorage(deps.Storage, deps.StorageMetrics)
	}
	if deps.NodeConfig.Bool(CfgPrometheusNode) {
		configureNode()
	}
	if deps.NodeConfig.Bool(CfgPrometheusGossip) {
		configureGossipPeers()
		configureGossipNode()
	}
	if deps.NodeConfig.Bool(CfgPrometheusCaches) {
		configureCaches()
	}
	if deps.NodeConfig.Bool(CfgPrometheusRestAPI) && deps.RestAPIMetrics != nil {
		configureRestAPI()
	}
	if deps.NodeConfig.Bool(CfgPrometheusINX) && deps.INXMetrics != nil {
		configureINX()
	}
	if deps.NodeConfig.Bool(CfgPrometheusINX) && deps.INXServer != nil {
		deps.INXServer.ConfigurePrometheus()
		registry.MustRegister(grpc_prometheus.DefaultServerMetrics)
	}
	if deps.NodeConfig.Bool(CfgPrometheusMigration) {
		if deps.ReceiptService != nil {
			configureReceipts()
		}
	}
	if deps.NodeConfig.Bool(CfgPrometheusDebug) {
		configureDebug()
	}
	if deps.NodeConfig.Bool(CfgPrometheusGoMetrics) {
		registry.MustRegister(collectors.NewGoCollector())
	}
	if deps.NodeConfig.Bool(CfgPrometheusProcessMetrics) {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
}

func addCollect(collect func()) {
	collects = append(collects, collect)
}

type fileservicediscovery struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func writeFileServiceDiscoveryFile() {
	path := deps.NodeConfig.String(CfgPrometheusFileServiceDiscoveryPath)
	d := []fileservicediscovery{{
		Targets: []string{deps.NodeConfig.String(CfgPrometheusFileServiceDiscoveryTarget)},
		Labels:  make(map[string]string),
	}}
	j, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		Plugin.LogPanic("unable to marshal file service discovery JSON:", err)
		return
	}

	// this truncates an existing file
	if err := ioutil.WriteFile(path, j, 0666); err != nil {
		Plugin.LogPanic("unable to write file service discovery file:", err)
	}

	Plugin.LogInfof("Wrote 'file service discovery' content to %s", path)
}

func run() {
	Plugin.LogInfo("Starting Prometheus exporter ...")

	if deps.NodeConfig.Bool(CfgPrometheusFileServiceDiscoveryEnabled) {
		writeFileServiceDiscoveryFile()
	}

	if err := Plugin.Daemon().BackgroundWorker("Prometheus exporter", func(ctx context.Context) {
		Plugin.LogInfo("Starting Prometheus exporter ... done")

		deps.PrometheusEcho.GET(RouteMetrics, func(c echo.Context) error {
			for _, collect := range collects {
				collect()
			}
			handler := promhttp.HandlerFor(
				registry,
				promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				},
			)
			if deps.NodeConfig.Bool(CfgPrometheusPromhttpMetrics) {
				handler = promhttp.InstrumentMetricHandler(registry, handler)
			}

			handler.ServeHTTP(c.Response().Writer, c.Request())
			return nil
		})

		bindAddr := deps.NodeConfig.String(CfgPrometheusBindAddress)

		go func() {
			Plugin.LogInfof("You can now access the Prometheus exporter using: http://%s/metrics", bindAddr)
			if err := deps.PrometheusEcho.Start(bindAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Plugin.LogWarnf("Stopped Prometheus exporter due to an error (%s)", err)
			}
		}()

		<-ctx.Done()
		Plugin.LogInfo("Stopping Prometheus exporter ...")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := deps.PrometheusEcho.Shutdown(shutdownCtx)
		if err != nil {
			Plugin.LogWarn(err)
		}
		shutdownCtxCancel()
		Plugin.LogInfo("Stopping Prometheus exporter ... done")
	}, shutdown.PriorityPrometheus); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}
