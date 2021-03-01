package prometheus

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the bind address on which the Prometheus exporter listens on
	CfgPrometheusBindAddress = "prometheus.bindAddress"
	// include go metrics
	CfgPrometheusGoMetrics = "prometheus.goMetrics"
	// include process metrics
	CfgPrometheusProcessMetrics = "prometheus.processMetrics"
	// include promhttp metrics
	CfgPrometheusPromhttpMetrics = "prometheus.promhttpMetrics"
	// whether the plugin should write a Prometheus 'file SD' file
	CfgPrometheusFileServiceDiscoveryEnabled = "prometheus.fileServiceDiscovery.enabled"
	// the path where to write the 'file SD' file to
	CfgPrometheusFileServiceDiscoveryPath = "prometheus.fileServiceDiscovery.path"
	// the target to write into the 'file SD' file
	CfgPrometheusFileServiceDiscoveryTarget = "prometheus.fileServiceDiscovery.target"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgPrometheusBindAddress, "localhost:9311", "the bind address on which the Prometheus exporter listens on")
			fs.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
			fs.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
			fs.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
			fs.Bool(CfgPrometheusFileServiceDiscoveryEnabled, false, "whether the plugin should write a Prometheus 'file SD' file")
			fs.String(CfgPrometheusFileServiceDiscoveryPath, "target.json", "the path where to write the 'file SD' file to")
			fs.String(CfgPrometheusFileServiceDiscoveryTarget, "localhost:9311", "the target to write into the 'file SD' file")
			return fs
		}(),
	},
	Masked: nil,
}
