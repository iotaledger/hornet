package config

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
	configFlagSet.String(CfgPrometheusBindAddress, "localhost:9311", "the bind address on which the Prometheus exporter listens on")
	configFlagSet.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
	configFlagSet.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
	configFlagSet.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
	configFlagSet.Bool(CfgPrometheusFileServiceDiscoveryEnabled, false, "whether the plugin should write a Prometheus 'file SD' file")
	configFlagSet.String(CfgPrometheusFileServiceDiscoveryPath, "target.json", "the path where to write the 'file SD' file to")
	configFlagSet.String(CfgPrometheusFileServiceDiscoveryTarget, "localhost:9311", "the target to write into the 'file SD' file")
}
