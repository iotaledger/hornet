package tipselection

import flag "github.com/spf13/pflag"

func init() {
	flag.Int("tipsel.maxDepth", 15, "Max. depth for tip selection")
	flag.Int("tipsel.belowMaxDepthTransactionLimit", 20000, "Number of tx to automatically flag them as below the max depth")
}
