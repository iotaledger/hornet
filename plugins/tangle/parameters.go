package tangle

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Path to the database folder"
	parameter.NodeConfig.SetDefault("db.path", "mainnetdb")

	// "Auto. set LSM as LSMI if enabled"
	parameter.NodeConfig.SetDefault("compass.loadLSMIAsLMI", false)
}
