package cli

const (
	// defines whether the node does PoW (e.g. if messages are received via API)
	CfgNodeEnableProofOfWork = "node.enableProofOfWork"
	// CfgNodeDisablePlugins defines a list of plugins that shall be disabled
	CfgNodeDisablePlugins = "node.disablePlugins"
	// CfgNodeEnablePlugins defines a list of plugins that shall be enabled
	CfgNodeEnablePlugins = "node.enablePlugins"
)

func init() {
	ConfigFlagSet.Bool(CfgNodeEnableProofOfWork, false, "defines whether the node does PoW (e.g. if messages are received via API)")
	ConfigFlagSet.StringSlice(CfgNodeDisablePlugins, nil, "a list of plugins that shall be disabled")
	ConfigFlagSet.StringSlice(CfgNodeEnablePlugins, nil, "a list of plugins that shall be enabled")
}
