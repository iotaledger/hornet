package spammer

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// the target address of the spam
	config.NodeConfig.SetDefault(config.CfgSpammerAddress, "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999")

	// the message to embed within the spam transactions
	config.NodeConfig.SetDefault(config.CfgSpammerMessage, "Spamming with HORNET tipselect")

	// the tag of the transaction
	config.NodeConfig.SetDefault(config.CfgSpammerTag, "HORNET99SPAMMER999999999999")

	// the depth to use for tip-selection
	config.NodeConfig.SetDefault(config.CfgSpammerDepth, 3)

	// the rate limit for the spammer (0 = no limit)
	config.NodeConfig.SetDefault(config.CfgSpammerTPSRateLimit, 0.10)

	// the amount of parallel running spammers
	config.NodeConfig.SetDefault(config.CfgSpammerWorkers, 1)
}
