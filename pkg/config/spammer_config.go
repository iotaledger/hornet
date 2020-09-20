package config

const (
	// the message to embed within the spam messages
	CfgSpammerMessage = "spammer.message"
	// the indexation of the message
	CfgSpammerIndex = "spammer.index"
	// the indexation of the message if the semi-lazy pool is used (uses "index" if empty)
	CfgSpammerIndexSemiLazy = "spammer.indexSemiLazy"
	// workers remains idle for a while when cpu usage gets over this limit (0 = disable)
	CfgSpammerCPUMaxUsage = "spammer.cpuMaxUsage"
	// the rate limit for the spammer (0 = no limit)
	CfgSpammerMPSRateLimit = "spammer.mpsRateLimit"
	// the amount of parallel running spammers
	CfgSpammerWorkers = "spammer.workers"
	// CfgSpammerAutostart automatically starts the spammer on node startup
	CfgSpammerAutostart = "spammer.autostart"
)

func init() {
	configFlagSet.String(CfgSpammerMessage, "Spamming with HORNET tipselect", "the message to embed within the spam messages")
	configFlagSet.String(CfgSpammerIndex, "HORNET99SPAMMER999999999999", "the indexation of the message")
	configFlagSet.String(CfgSpammerIndexSemiLazy, "", "the indexation of the message if the semi-lazy pool is used (uses \"index\" if empty)")
	configFlagSet.Float64(CfgSpammerCPUMaxUsage, 0.50, "workers remains idle for a while when cpu usage gets over this limit (0 = disable)")
	configFlagSet.Float64(CfgSpammerMPSRateLimit, 0.10, "the rate limit for the spammer (0 = no limit)")
	configFlagSet.Int(CfgSpammerWorkers, 1, "the amount of parallel running spammers")
	configFlagSet.Bool(CfgSpammerAutostart, false, "automatically start the spammer on node startup")
}
