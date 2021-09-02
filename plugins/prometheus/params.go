package prometheus

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the bind address on which the Prometheus exporter listens on.
	CfgPrometheusBindAddress = "prometheus.bindAddress"
	// whether the plugin should write a Prometheus 'file SD' file.
	CfgPrometheusFileServiceDiscoveryEnabled = "prometheus.fileServiceDiscovery.enabled"
	// the path where to write the 'file SD' file to.
	CfgPrometheusFileServiceDiscoveryPath = "prometheus.fileServiceDiscovery.path"
	// the target to write into the 'file SD' file.
	CfgPrometheusFileServiceDiscoveryTarget = "prometheus.fileServiceDiscovery.target"
	// include database metrics.
	CfgPrometheusDatabase = "prometheus.databaseMetrics"
	// include node metrics.
	CfgPrometheusNode = "prometheus.nodeMetrics"
	// include gossip metrics.
	CfgPrometheusGossip = "prometheus.gossipMetrics"
	// include caches metrics.
	CfgPrometheusCaches = "prometheus.cachesMetrics"
	// include restAPI metrics.
	CfgPrometheusRestAPI = "prometheus.restAPIMetrics"
	// include migration metrics.
	CfgPrometheusMigration = "prometheus.migrationMetrics"
	// include coordinator metrics.
	CfgPrometheusCoordinator = "prometheus.coordinatorMetrics"
	// include debug metrics.
	CfgPrometheusDebug = "prometheus.debugMetrics"
	// include go metrics.
	CfgPrometheusGoMetrics = "prometheus.goMetrics"
	// include process metrics.
	CfgPrometheusProcessMetrics = "prometheus.processMetrics"
	// include promhttp metrics.
	CfgPrometheusPromhttpMetrics = "prometheus.promhttpMetrics"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgPrometheusBindAddress, "localhost:9311", "the bind address on which the Prometheus exporter listens on")
			fs.Bool(CfgPrometheusFileServiceDiscoveryEnabled, false, "whether the plugin should write a Prometheus 'file SD' file")
			fs.String(CfgPrometheusFileServiceDiscoveryPath, "target.json", "the path where to write the 'file SD' file to")
			fs.String(CfgPrometheusFileServiceDiscoveryTarget, "localhost:9311", "the target to write into the 'file SD' file")
			fs.Bool(CfgPrometheusDatabase, true, "include database metrics")
			fs.Bool(CfgPrometheusNode, true, "include node metrics")
			fs.Bool(CfgPrometheusGossip, true, "include gossip metrics")
			fs.Bool(CfgPrometheusCaches, true, "include caches metrics")
			fs.Bool(CfgPrometheusRestAPI, true, "include restAPI metrics")
			fs.Bool(CfgPrometheusMigration, true, "include migration metrics")
			fs.Bool(CfgPrometheusCoordinator, true, "include coordinator metrics")
			fs.Bool(CfgPrometheusDebug, false, "include debug metrics")
			fs.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
			fs.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
			fs.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
			return fs
		}(),
	},
	Masked: nil,
}
