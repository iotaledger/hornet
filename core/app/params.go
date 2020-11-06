package app

import (
	flag "github.com/spf13/pflag"
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

func init() {
	flag.StringSlice(CfgNodeDisablePlugins, nil, "a list of plugins that shall be disabled")
	flag.StringSlice(CfgNodeEnablePlugins, nil, "a list of plugins that shall be enabled")
}
