package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the target address of the spam
	CfgSpammerAddress = "spammer.address"
	// the message to embed within the spam transactions
	CfgSpammerMessage = "spammer.message"
	// the tag of the transaction
	CfgSpammerTag = "spammer.tag"
	// the depth to use for tip-selection
	CfgSpammerDepth = "spammer.depth"
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
)

func init() {
	flag.String(CfgSpammerAddress, "HORNET99INTEGRATED99SPAMMER999999999999999999999999999999999999999999999999999999", "the target address of the spam")
	flag.String(CfgSpammerMessage, "Spamming with HORNET tipselect", "the message to embed within the spam transactions")
	flag.String(CfgSpammerTag, "HORNET99SPAMMER999999999999", "the tag of the transaction")
	flag.Int(CfgSpammerDepth, 3, "the depth to use for tip-selection")
	flag.Float64(CfgSpammerCPUMaxUsage, 0.50, "workers remains idle for a while when cpu usage gets over this limit (0 = disable)")
	flag.Float64(CfgSpammerTPSRateLimit, 0.10, "the rate limit for the spammer (0 = no limit)")
	flag.Int(CfgSpammerBundleSize, 1, "the size of the spam bundles")
	flag.Bool(CfgSpammerValueSpam, false, "should be spammed with value bundles")
	flag.Int(CfgSpammerWorkers, 1, "the amount of parallel running spammers")
}
