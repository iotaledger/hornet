package config

const (
	// the max allowed depth to be used as the starting point for tip-selection
	CfgTipSelMaxDepth = "tipsel.maxDepth"
	// the limit defining the max amount of transactions to traverse in order to check
	// whether a transaction references a transaction below max depth
	CfgTipSelBelowMaxDepthTransactionLimit = "tipsel.belowMaxDepthTransactionLimit"
)

func init() {
	NodeConfig.SetDefault(CfgTipSelMaxDepth, 15)
	NodeConfig.SetDefault(CfgTipSelBelowMaxDepthTransactionLimit, 20000)
}
