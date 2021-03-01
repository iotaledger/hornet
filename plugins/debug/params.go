package debug

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the maximum duration for the parents to become solid during white flag confirmation API call.
	CfgDebugWhiteFlagParentsSolidTimeout = "debug.whiteFlagParentsSolidTimeout"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Duration(CfgDebugWhiteFlagParentsSolidTimeout, 2*time.Second, "defines the the maximum duration for the parents to become solid during white flag confirmation API call")
			return fs
		}(),
	},
	Masked: nil,
}
