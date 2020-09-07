package config

const (
	// the target address of the spam
	CfgSpammerAddress = "spammer.address"
	// the message to embed within the spam transactions
	CfgSpammerMessage = "spammer.message"
	// the tag of the transaction
	CfgSpammerTag = "spammer.tag"
	// the tag of the transaction if the semi-lazy pool is used (uses "tag" if empty)
	CfgSpammerTagSemiLazy = "spammer.tagSemiLazy"
	// workers remains idle for a while when cpu usage gets over this limit (0 = disable)
	CfgSpammerCPUMaxUsage = "spammer.cpuMaxUsage"
	// the rate limit for the spammer (0 = no limit)
	CfgSpammerTPSRateLimit = "spammer.tpsRateLimit"
	// the size of the spam bundles
	CfgSpammerBundleSize = "spammer.bundleSize"
	// should be spammed with value bundles
	CfgSpammerValueSpam = "spammer.valueSpam"
	// the amount of parallel running spammers
	CfgSpammerWorkers = "spammer.workers"
	// CfgSpammerAutostart automatically starts the spammer on node startup
	CfgSpammerAutostart = "spammer.autostart"
)

func init() {
	configFlagSet.String(CfgSpammerAddress, "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999", "the target address of the spam")
	configFlagSet.String(CfgSpammerMessage, "Spamming with HORNET tipselect", "the message to embed within the spam transactions")
	configFlagSet.String(CfgSpammerTag, "HORNET99SPAMMER999999999999", "the tag of the transaction")
	configFlagSet.String(CfgSpammerTagSemiLazy, "", "the tag of the transaction if the semi-lazy pool is used (uses \"tag\" if empty)")
	configFlagSet.Float64(CfgSpammerCPUMaxUsage, 0.50, "workers remains idle for a while when cpu usage gets over this limit (0 = disable)")
	configFlagSet.Float64(CfgSpammerTPSRateLimit, 0.10, "the rate limit for the spammer (0 = no limit)")
	configFlagSet.Int(CfgSpammerBundleSize, 1, "the size of the spam bundles")
	configFlagSet.Bool(CfgSpammerValueSpam, false, "should be spammed with value bundles")
	configFlagSet.Int(CfgSpammerWorkers, 1, "the amount of parallel running spammers")
	configFlagSet.Bool(CfgSpammerAutostart, false, "automatically start the spammer on node startup")
}
