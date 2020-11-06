package app

import (
	flag "github.com/spf13/pflag"
)

const (
	// defines whether the node does PoW (e.g. if messages are received via API)
	CfgNodeEnableProofOfWork = "node.enableProofOfWork"
	// CfgNodeDisablePlugins defines a list of plugins that shall be disabled
	CfgNodeDisablePlugins = "node.disablePlugins"
	// CfgNodeEnablePlugins defines a list of plugins that shall be enabled
	CfgNodeEnablePlugins = "node.enablePlugins"

	CfgConfigFilePathNodeConfig     = "config"
	CfgConfigFilePathPeeringConfig  = "peering"
	CfgConfigFilePathProfilesConfig = "profiles"
)

func init() {
	flag.Bool(CfgNodeEnableProofOfWork, false, "defines whether the node does PoW (e.g. if messages are received via API)")
	flag.StringSlice(CfgNodeDisablePlugins, nil, "a list of plugins that shall be disabled")
	flag.StringSlice(CfgNodeEnablePlugins, nil, "a list of plugins that shall be enabled")
}
