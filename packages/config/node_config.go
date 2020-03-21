package config

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.Alias"
	// CfgNodeShowAliasInGetNodeInfo defines whether to show the alias in getNodeInfo
	CfgNodeShowAliasInGetNodeInfo = "node.showAliasInGetNodeInfo"
)

func init() {
	NodeConfig.SetDefault(CfgNodeAlias, "")
	NodeConfig.SetDefault(CfgNodeShowAliasInGetNodeInfo, false)
}
