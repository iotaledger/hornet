package profiling

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// the bind address on which the profiler listens on
	CfgProfilingBindAddress = "profiling.bindAddress"
)

func init() {
	cli.ConfigFlagSet.String(CfgProfilingBindAddress, "localhost:6060", "the bind address on which the profiler listens on")
}
