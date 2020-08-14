package prometheus

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/shutdown"
)

// PLUGIN Prometheus
var (
	PLUGIN = node.NewPlugin("Prometheus", node.Disabled, configure, run)
	log    *logger.Logger

	server   *http.Server
	registry = prometheus.NewRegistry()
	collects []func()
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	if config.NodeConfig.GetBool(config.CfgPrometheusGoMetrics) {
		registry.MustRegister(prometheus.NewGoCollector())
	}
	if config.NodeConfig.GetBool(config.CfgPrometheusProcessMetrics) {
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
	path := config.NodeConfig.GetString(config.CfgPrometheusFileServiceDiscoveryPath)
	d := []fileservicediscovery{{
		Targets: []string{config.NodeConfig.GetString(config.CfgPrometheusFileServiceDiscoveryTarget)},
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

func run(plugin *node.Plugin) {
	log.Info("Starting Prometheus exporter ...")

	if config.NodeConfig.GetBool(config.CfgPrometheusFileServiceDiscoveryEnabled) {
		writeFileServiceDiscoveryFile()
	}

	daemon.BackgroundWorker("Prometheus exporter", func(shutdownSignal <-chan struct{}) {
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
			if config.NodeConfig.GetBool(config.CfgPrometheusPromhttpMetrics) {
				handler = promhttp.InstrumentMetricHandler(registry, handler)
			}
			handler.ServeHTTP(c.Writer, c.Request)
		})

		bindAddr := config.NodeConfig.GetString(config.CfgPrometheusBindAddress)
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
