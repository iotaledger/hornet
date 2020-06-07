package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the bind address on which the profiler listens on
	CfgProfilingBindAddress = "profiling.bindAddress"
)

func init() {
	flag.String(CfgProfilingBindAddress, "localhost:6060", "the bind address on which the profiler listens on")
}
