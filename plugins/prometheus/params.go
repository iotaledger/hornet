package prometheus

import (
	"github.com/iotaledger/hive.go/app"
)

// ParametersPrometheus contains the definition of the parameters used by WarpSync.
type ParametersPrometheus struct {
	// the bind address on which the Prometheus exporter listens on.
	BindAddress string `default:"localhost:9311" usage:"the bind address on which the Prometheus exporter listens on"`

	FileServiceDiscovery struct {
		// whether the plugin should write a Prometheus 'file SD' file.
		Enabled bool `default:"false" usage:"whether the plugin should write a Prometheus 'file SD' file"`
		// the path where to write the 'file SD' file to.
		Path string `default:"target.json" usage:"the path where to write the 'file SD' file to"`
		// the target to write into the 'file SD' file.
		Target string `default:"localhost:9311" usage:"the target to write into the 'file SD' file"`
	}

	// include database metrics.
	DatabaseMetrics bool `default:"true" usage:"include database metrics"`
	// include node metrics.
	NodeMetrics bool `default:"true" usage:"include node metrics"`
	// include gossip metrics.
	GossipMetrics bool `default:"true" usage:"include gossip metrics"`
	// include caches metrics.
	CachesMetrics bool `default:"true" usage:"include caches metrics"`
	// include restAPI metrics.
	RestAPIMetrics bool `default:"true" usage:"include restAPI metrics"`
	// include INXMetrics metrics.
	INXMetrics bool `default:"true" usage:"include INX metrics"`
	// include migration metrics.
	MigrationMetrics bool `default:"true" usage:"include migration metrics"`
	// include debug metrics.
	DebugMetrics bool `default:"false" usage:"include debug metrics"`
	// include go metrics.
	GoMetrics bool `default:"false" usage:"include go metrics"`
	// include process metrics.
	ProcessMetrics bool `default:"false" usage:"include process metrics"`
	// include promhttp metrics.
	PromhttpMetrics bool `default:"false" usage:"include promhttp metrics"`
}

var ParamsPrometheus = &ParametersPrometheus{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"prometheus": ParamsPrometheus,
	},
	Masked: nil,
}
