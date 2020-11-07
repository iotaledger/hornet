package profiling

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// the bind address on which the profiler listens on
	CfgProfilingBindAddress = "profiling.bindAddress"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgProfilingBindAddress, "localhost:6060", "the bind address on which the profiler listens on")
			return fs
		}(),
	},
	Masked: nil,
}
