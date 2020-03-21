package config

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.Alias"
)

func init() {
	NodeConfig.SetDefault(CfgNodeAlias, "")
}
