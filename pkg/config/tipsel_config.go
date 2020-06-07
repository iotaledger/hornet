package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the max allowed depth to be used as the starting point for tip-selection
	CfgTipSelMaxDepth = "tipsel.maxDepth"
	// the limit defining the max amount of transactions to traverse in order to check
	// whether a transaction references a transaction below max depth
	CfgTipSelBelowMaxDepthTransactionLimit = "tipsel.belowMaxDepthTransactionLimit"
)

func init() {
	flag.Int(CfgTipSelMaxDepth, 3, "the max allowed depth to be used as the starting point for tip-selection")
	flag.Int(CfgTipSelBelowMaxDepthTransactionLimit, 20000, "the limit defining the max amount of transactions to traverse in order to check "+
		"whether a transaction references a transaction below max depth")
}
