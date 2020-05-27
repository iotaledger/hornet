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
)

func init() {
	NodeConfig.SetDefault(CfgPrometheusBindAddress, "localhost:9311")
	NodeConfig.SetDefault(CfgPrometheusGoMetrics, false)
	NodeConfig.SetDefault(CfgPrometheusProcessMetrics, false)
	NodeConfig.SetDefault(CfgPrometheusPromhttpMetrics, false)
}
