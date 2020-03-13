package profiling

import "github.com/gohornet/hornet/packages/parameter"

func init() {
	// The bind address on which the profiler listens on
	parameter.NodeConfig.SetDefault("profiling.bindAddress", "localhost:6060")
}
