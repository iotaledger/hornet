package inx

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgINXBindAddress the bind address on which the INX can be accessed from
	CfgINXBindAddress = "inx.bindAddress"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgINXBindAddress, "localhost:9029", "the bind address on which the INX can be accessed from")
			return fs
		}(),
	},
}
