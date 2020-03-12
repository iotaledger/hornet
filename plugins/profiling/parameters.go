package profiling

import "github.com/gohornet/hornet/packages/parameter"

func init() {
	// The bind address on which the profiler listens on
	parameter.NodeConfig.SetDefault("profiling.bindAddress", "localhost:6060")
	parameter.NodeConfig.SetDefault("profiling.bindAddress", "127.0.0.1")

	// The port on which the profiler listens for request
	parameter.NodeConfig.SetDefault("profiling.port", 6060)
}
