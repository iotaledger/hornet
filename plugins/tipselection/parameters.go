package tipselection

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// the max allowed depth to be used as the starting point for tip-selection
	config.NodeConfig.SetDefault(config.CfgTipSelMaxDepth, 15)

	// the limit defining the max amount of transactions to traverse in order to check
	// whether a transaction references a transaction below max depth
	config.NodeConfig.SetDefault(config.CfgTipSelBelowMaxDepthTransactionLimit, 20000)
}
