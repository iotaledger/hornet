package config

import (
	flag "github.com/spf13/pflag"
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
)

func init() {
	flag.String(CfgPrometheusBindAddress, "localhost:9311", "the bind address on which the Prometheus exporter listens on")
	flag.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
	flag.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
	flag.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
}
