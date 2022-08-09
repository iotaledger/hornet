package prometheus

import (
	"github.com/iotaledger/hive.go/core/app"
)

// ParametersPrometheus contains the definition of the parameters used by Prometheus.
type ParametersPrometheus struct {
	// Enabled defines whether the prometheus plugin is enabled.
	Enabled bool `default:"false" usage:"whether the prometheus plugin is enabled"`
	// defines the bind address on which the Prometheus exporter listens on.
	BindAddress string `default:"localhost:9311" usage:"the bind address on which the Prometheus exporter listens on"`

	FileServiceDiscovery struct {
		// Enabled defines whether the plugin should write a Prometheus 'file SD' file.
		Enabled bool `default:"false" usage:"whether the plugin should write a Prometheus 'file SD' file"`
		// Path defines the path where to write the 'file SD' file to.
		Path string `default:"target.json" usage:"the path where to write the 'file SD' file to"`
		// Target defines the target to write into the 'file SD' file.
		Target string `default:"localhost:9311" usage:"the target to write into the 'file SD' file"`
	}

	// DatabaseMetrics defines whether to include database metrics.
	DatabaseMetrics bool `default:"true" usage:"whether to include database metrics"`
	// NodeMetrics defines whether to include node metrics.
	NodeMetrics bool `default:"true" usage:"whether to include node metrics"`
	// GossipMetrics defines whether to include gossip metrics.
	GossipMetrics bool `default:"true" usage:"whether to include gossip metrics"`
	// CachesMetrics defines whether to include caches metrics.
	CachesMetrics bool `default:"true" usage:"whether to include caches metrics"`
	// RestAPIMetrics include restAPI metrics.
	RestAPIMetrics bool `default:"true" usage:"whether to include restAPI metrics"`
	// INXMetrics defines whether to include INXMetrics metrics.
	INXMetrics bool `name:"inxMetrics" default:"true" usage:"whether to include INX metrics"`
	// MigrationMetrics defines whether to include migration metrics.
	MigrationMetrics bool `default:"true" usage:"whether to include migration metrics"`
	// DebugMetrics defines whether to include debug metrics.
	DebugMetrics bool `default:"false" usage:"whether to include debug metrics"`
	// GoMetrics defines whether to include go metrics.
	GoMetrics bool `default:"false" usage:"whether to include go metrics"`
	// ProcessMetrics defines whether to include process metrics.
	ProcessMetrics bool `default:"false" usage:"whether to include process metrics"`
	// PromhttpMetrics defines whether to include promhttp metrics.
	PromhttpMetrics bool `default:"false" usage:"whether to include promhttp metrics"`
}

var ParamsPrometheus = &ParametersPrometheus{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"prometheus": ParamsPrometheus,
	},
	Masked: nil,
}
