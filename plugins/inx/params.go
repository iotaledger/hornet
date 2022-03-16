package inx

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgINXPort the port the INX extensions should connect to
	CfgINXPort = "inx.port"
	// CfxINXPath the path with the extensions to be loaded
	CfxINXPath = "inx.path"
	// CfgINXExtensionLogsEnabled controls if stdout/stderr of each extension should be written to a log file
	CfgINXExtensionLogsEnabled = "inx.extensionLogsEnabled"
	// CfgINXDisableExtensions defines a list of extensions that shall be disabled
	CfgINXDisableExtensions = "inx.disableExtensions"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgINXPort, 9029, "the port the INX extensions should connect to")
			fs.String(CfxINXPath, "inx", "the path with the extensions to be loaded")
			fs.Bool(CfgINXExtensionLogsEnabled, false, "controls if stdout/stderr of each extension should be written to a log file")
			fs.StringSlice(CfgINXDisableExtensions, []string{}, "defines a list of extensions that shall be disabled")
			return fs
		}(),
	},
}
