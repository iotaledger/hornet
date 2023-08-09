package config

const (
	// the bind address on which the profiler listens on
	CfgProfilingBindAddress = "profiling.bindAddress"
)

func init() {
	configFlagSet.String(CfgProfilingBindAddress, "localhost:6061", "the bind address on which the profiler listens on")
}
