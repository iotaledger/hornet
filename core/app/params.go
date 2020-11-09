package app

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgNodeDisablePlugins defines a list of plugins that shall be disabled
	CfgNodeDisablePlugins = "node.disablePlugins"
	// CfgNodeEnablePlugins defines a list of plugins that shall be enabled
	CfgNodeEnablePlugins = "node.enablePlugins"

	CfgConfigFilePathNodeConfig     = "config"
	CfgConfigFilePathPeeringConfig  = "peering"
	CfgConfigFilePathProfilesConfig = "profiles"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.StringSlice(CfgNodeDisablePlugins, nil, "a list of plugins that shall be disabled")
			fs.StringSlice(CfgNodeEnablePlugins, nil, "a list of plugins that shall be enabled")
			return fs
		}(),
	},
	Masked: nil,
}
