package prometheus

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	Plugin *node.Plugin
	log    *logger.Logger

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()

	deps dependencies
)

type dependencies struct {
	dig.In
	NodeConfig   *configuration.Configuration `name:"nodeConfig"`
	Tangle       *tangle.Tangle
	Service      *gossip.Service
	Manager      *p2p.Manager
	RequestQueue gossip.RequestQueue
}

func init() {
	Plugin = node.NewPlugin("Prometheus", node.Disabled, configure, run)
}

func configure(c *dig.Container) {
	log = logger.NewLogger(Plugin.Name)

	if err := c.Invoke(func(cDeps dependencies) {
		deps = cDeps
	}); err != nil {
		panic(err)
	}

	if deps.NodeConfig.Bool(config.CfgPrometheusGoMetrics) {
		registry.MustRegister(prometheus.NewGoCollector())
	}
	if deps.NodeConfig.Bool(config.CfgPrometheusProcessMetrics) {
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
	path := deps.NodeConfig.String(config.CfgPrometheusFileServiceDiscoveryPath)
	d := []fileservicediscovery{{
		Targets: []string{deps.NodeConfig.String(config.CfgPrometheusFileServiceDiscoveryTarget)},
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

func run(_ *dig.Container) {
	log.Info("Starting Prometheus exporter ...")

	if deps.NodeConfig.Bool(config.CfgPrometheusFileServiceDiscoveryEnabled) {
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
			if deps.NodeConfig.Bool(config.CfgPrometheusPromhttpMetrics) {
				handler = promhttp.InstrumentMetricHandler(registry, handler)
			}
			handler.ServeHTTP(c.Writer, c.Request)
		})

		bindAddr := deps.NodeConfig.String(config.CfgPrometheusBindAddress)
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
