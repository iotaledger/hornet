package tipselection

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Max. depth for tip selection"
	parameter.NodeConfig.SetDefault("tipsel.maxDepth", 15)

	// "Number of tx to automatically flag them as below the max depth"
	parameter.NodeConfig.SetDefault("tipsel.belowMaxDepthTransactionLimit", 20000)
}
