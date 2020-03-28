package config

const (
	// the bind address on which the profiler listens on
	CfgProfilingBindAddress = "profiling.bindAddress"
)

func init() {
	NodeConfig.SetDefault(CfgProfilingBindAddress, "localhost:6060")
}
