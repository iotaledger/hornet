package prometheus

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/app"
	databasepkg "github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
)

// RouteMetrics is the route for getting the prometheus metrics.
// GET returns metrics.
const (
	RouteMetrics = "/metrics"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Disabled,
		Pluggable: node.Pluggable{
			Name:      "Prometheus",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	log    *logger.Logger
	deps   dependencies

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()
)

type dependencies struct {
	dig.In
	AppInfo          *app.AppInfo
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
	Database         *databasepkg.Database
	Storage          *storage.Storage
	ServerMetrics    *metrics.ServerMetrics
	DatabaseMetrics  *metrics.DatabaseMetrics
	StorageMetrics   *metrics.StorageMetrics
	RestAPIMetrics   *metrics.RestAPIMetrics `optional:"true"`
	Service          *gossip.Service
	ReceiptService   *migrator.ReceiptService `optional:"true"`
	Tangle           *tangle.Tangle
	MigratorService  *migrator.MigratorService `optional:"true"`
	Manager          *p2p.Manager
	RequestQueue     gossip.RequestQueue
	MessageProcessor *gossip.MessageProcessor
	TipSelector      *tipselect.TipSelector
	Snapshot         *snapshot.Snapshot
	Coordinator      *coordinator.Coordinator `optional:"true"`
	DatabaseEvents   *database.Events
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	if deps.NodeConfig.Bool(CfgPrometheusDatabase) {
		configureDatabase()
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
	if deps.NodeConfig.Bool(CfgPrometheusMigration) {
		if deps.ReceiptService != nil {
			configureReceipts()
		}
		if deps.MigratorService != nil {
			configureMigrator()
		}
	}
	if deps.NodeConfig.Bool(CfgPrometheusCoordinator) && deps.Coordinator != nil {
		configureCoordinator()
	}
	if deps.NodeConfig.Bool(CfgPrometheusDebug) {
		configureDebug()
	}
	if deps.NodeConfig.Bool(CfgPrometheusGoMetrics) {
		registry.MustRegister(prometheus.NewGoCollector())
	}
	if deps.NodeConfig.Bool(CfgPrometheusProcessMetrics) {
		registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
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
		log.Panic("unable to marshal file service discovery JSON:", err)
		return
	}

	// this truncates an existing file
	if err := ioutil.WriteFile(path, j, 0666); err != nil {
		log.Panic("unable to write file service discovery file:", err)
	}

	log.Infof("Wrote 'file service discovery' content to %s", path)
}

func run() {
	log.Info("Starting Prometheus exporter ...")

	if deps.NodeConfig.Bool(CfgPrometheusFileServiceDiscoveryEnabled) {
		writeFileServiceDiscoveryFile()
	}

	Plugin.Daemon().BackgroundWorker("Prometheus exporter", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Prometheus exporter ... done")

		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())

		e.GET(RouteMetrics, func(c echo.Context) error {
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
		server = &http.Server{Addr: bindAddr, Handler: e}

		go func() {
			log.Infof("You can now access the Prometheus exporter using: http://%s/metrics", bindAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Warn("Stopping Prometheus exporter due to an error ... done")
			}
		}()

		<-shutdownSignal
		log.Info("Stopping Prometheus exporter ...")

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := server.Shutdown(ctx)
			if err != nil {
				log.Warn(err.Error())
			}
			cancel()
		}
		log.Info("Stopping Prometheus exporter ... done")
	}, shutdown.PriorityPrometheus)
}
