package config

const (
	// the target address of the spam
	CfgSpammerAddress = "spammer.address"
	// the message to embed within the spam transactions
	CfgSpammerMessage = "spammer.message"
	// the tag of the transaction
	CfgSpammerTag = "spammer.tag"
	// the depth to use for tip-selection
	CfgSpammerDepth = "spammer.depth"
	// the rate limit for the spammer (0 = no limit)
	CfgSpammerTPSRateLimit = "spammer.tpsRateLimit"
	// the amount of parallel running spammers
	CfgSpammerWorkers = "spammer.workers"
)

func init() {
	NodeConfig.SetDefault(CfgSpammerAddress, "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999")
	NodeConfig.SetDefault(CfgSpammerMessage, "Spamming with HORNET tipselect")
	NodeConfig.SetDefault(CfgSpammerTag, "HORNET99SPAMMER999999999999")
	NodeConfig.SetDefault(CfgSpammerDepth, 3)
	NodeConfig.SetDefault(CfgSpammerTPSRateLimit, 0.10)
	NodeConfig.SetDefault(CfgSpammerWorkers, 1)
}
