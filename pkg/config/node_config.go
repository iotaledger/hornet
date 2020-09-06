package config

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.alias"
	// CfgNodeShowAliasInGetNodeInfo defines whether to show the alias in getNodeInfo
	CfgNodeShowAliasInGetNodeInfo = "node.showAliasInGetNodeInfo"
)

func init() {
	configFlagSet.String(CfgNodeAlias, "", "set an alias to identify a node")
	configFlagSet.Bool(CfgNodeShowAliasInGetNodeInfo, false, "defines whether to show the alias in getNodeInfo")
}
