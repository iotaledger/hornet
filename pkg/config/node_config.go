package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgNodeAlias set an alias to identify a node
	CfgNodeAlias = "node.alias"
	// CfgNodeShowAliasInGetNodeInfo defines whether to show the alias in getNodeInfo
	CfgNodeShowAliasInGetNodeInfo = "node.showAliasInGetNodeInfo"
)

func init() {
	flag.String(CfgNodeAlias, "", "set an alias to identify a node")
	flag.Bool(CfgNodeShowAliasInGetNodeInfo, false, "defines whether to show the alias in getNodeInfo")
}
