package config

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.alias"
	// CfgNodeShowAliasInGetNodeInfo defines whether to show the alias in getNodeInfo
	CfgNodeShowAliasInGetNodeInfo = "node.showAliasInGetNodeInfo"
	// CfgNodeDisablePlugins defines a list of plugins that shall be disabled
	CfgNodeDisablePlugins = "node.disablePlugins"
	// CfgNodeEnablePlugins defines a list of plugins that shall be enabled
	CfgNodeEnablePlugins = "node.enablePlugins"
)

func init() {
	configFlagSet.String(CfgNodeAlias, "", "set an alias to identify a node")
	configFlagSet.Bool(CfgNodeShowAliasInGetNodeInfo, false, "defines whether to show the alias in getNodeInfo")
	configFlagSet.StringSlice(CfgNodeDisablePlugins, nil, "a list of plugins that shall be disabled")
	configFlagSet.StringSlice(CfgNodeEnablePlugins, nil, "a list of plugins that shall be enabled")
}
