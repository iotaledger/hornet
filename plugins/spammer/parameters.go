package spammer

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Tx Address"
	parameter.NodeConfig.SetDefault("spammer.address", "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999")

	// "Message of the Tx"
	parameter.NodeConfig.SetDefault("spammer.message", "Spamming with HORNET tipselect")

	// "Tag of the Tx"
	parameter.NodeConfig.SetDefault("spammer.tag", "HORNET99SPAMMER999999999999")

	// "Depth of the random walker"
	parameter.NodeConfig.SetDefault("spammer.depth", 3)

	// "Rate limit for the spam (0 = no limit)"
	parameter.NodeConfig.SetDefault("spammer.tpsRateLimit", 0.10)

	// "How many spammers should run in parallel"
	parameter.NodeConfig.SetDefault("spammer.workers", 1)
}
