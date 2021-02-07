package debug

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// the maximum duration for the parents to become solid during white flag confirmation API call.
	CfgDebugWhiteFlagParentsSolidTimeoutMilliseconds = "debug.whiteFlagParentsSolidTimeoutMilliseconds"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgDebugWhiteFlagParentsSolidTimeoutMilliseconds, 2000, "defines the the maximum duration for the parents to become solid during white flag confirmation API call")
			return fs
		}(),
	},
	Masked: nil,
}
