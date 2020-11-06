package prometheus

import (
	"github.com/gohornet/hornet/core/cli"
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

func init() {
	cli.ConfigFlagSet.String(CfgPrometheusBindAddress, "localhost:9311", "the bind address on which the Prometheus exporter listens on")
	cli.ConfigFlagSet.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
	cli.ConfigFlagSet.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
	cli.ConfigFlagSet.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
	cli.ConfigFlagSet.Bool(CfgPrometheusFileServiceDiscoveryEnabled, false, "whether the plugin should write a Prometheus 'file SD' file")
	cli.ConfigFlagSet.String(CfgPrometheusFileServiceDiscoveryPath, "target.json", "the path where to write the 'file SD' file to")
	cli.ConfigFlagSet.String(CfgPrometheusFileServiceDiscoveryTarget, "localhost:9311", "the target to write into the 'file SD' file")
}
