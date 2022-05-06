package profiling

import (
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// the bind address on which the profiler listens on
	CfgProfilingBindAddress = "profiling.bindAddress"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.String(CfgProfilingBindAddress, "localhost:6060", "the bind address on which the profiler listens on")
	},
	Masked: nil,
}
