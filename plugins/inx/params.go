package inx

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the port the INX extensions should connect to
	CfgINXPort = "inx.port"

	// the path with the extensions to be loaded
	CfxINXPath = "inx.path"

	// defines a list of extensions that shall be disabled
	CfgINXDisableExtensions = "inx.disableExtensions"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgINXPort, 9029, "the port the INX extensions should connect to")
			fs.String(CfxINXPath, "inx", "the path with the extensions to be loaded")
			fs.StringSlice(CfgINXDisableExtensions, []string{}, "defines a list of extensions that shall be disabled")
			return fs
		}(),
	},
}
