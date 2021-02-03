package prometheus

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
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
	AppInfo         *app.AppInfo
	NodeConfig      *configuration.Configuration `name:"nodeConfig"`
	Storage         *storage.Storage
	ServerMetrics   *metrics.ServerMetrics
	Service         *gossip.Service
	ReceiptService  *migrator.ReceiptService  `optional:"true"`
	MigratorService *migrator.MigratorService `optional:"true"`
	Manager         *p2p.Manager
	RequestQueue    gossip.RequestQueue
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	configureData()
	configureInfo()
	configurePeers()
	configureServer()
	if deps.MigratorService != nil {
		// TODO: enable this afterwards
		//configureMigrator()
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

		engine := gin.New()
		engine.Use(gin.Recovery())
		engine.GET("/metrics", func(c *gin.Context) {
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
			handler.ServeHTTP(c.Writer, c.Request)
		})

		bindAddr := deps.NodeConfig.String(CfgPrometheusBindAddress)
		server = &http.Server{Addr: bindAddr, Handler: engine}

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
