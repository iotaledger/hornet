package tangle

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// the path to the database folder
	config.NodeConfig.SetDefault(config.CfgDatabasePath, "mainnetdb")

	// whether to auto. set LSM as LSMI
	config.NodeConfig.SetDefault(config.CfgCompassLoadLSMIAsLMI, false)
}
