package warpsync

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the used advancement range per warpsync checkpoint
	CfgWarpSyncAdvancementRange = "warpsync.advancementRange"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgWarpSyncAdvancementRange, 150, "the used advancement range per warpsync checkpoint")
			return fs
		}(),
	},
	Masked: nil,
}
