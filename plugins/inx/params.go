package inx

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgINXPort the port the INX extensions should connect to
	CfgINXPort = "inx.port"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgINXPort, 9029, "the port the INX extensions should connect to")
			return fs
		}(),
	},
}
