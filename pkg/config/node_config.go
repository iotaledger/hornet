package config

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.alias"
	// defines whether the node does PoW (e.g. if messages are received via API)
	CfgNodeEnableProofOfWork = "node.enableProofOfWork"
	// CfgNodeDisablePlugins defines a list of plugins that shall be disabled
	CfgNodeDisablePlugins = "node.disablePlugins"
	// CfgNodeEnablePlugins defines a list of plugins that shall be enabled
	CfgNodeEnablePlugins = "node.enablePlugins"
)

func init() {
	configFlagSet.String(CfgNodeAlias, "", "set an alias to identify a node")
	configFlagSet.Bool(CfgNodeEnableProofOfWork, false, "defines whether the node does PoW (e.g. if messages are received via API)")
	configFlagSet.StringSlice(CfgNodeDisablePlugins, nil, "a list of plugins that shall be disabled")
	configFlagSet.StringSlice(CfgNodeEnablePlugins, nil, "a list of plugins that shall be enabled")
}
