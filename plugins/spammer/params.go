package spammer

import (
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app"
)

const (
	// the message to embed within the spam messages
	CfgSpammerMessage = "spammer.message"
	// the tag of the message
	CfgSpammerTag = "spammer.tag"
	// the tag of the message if the semi-lazy pool is used (uses "tag" if empty)
	CfgSpammerTagSemiLazy = "spammer.tagSemiLazy"
	// workers remains idle for a while when cpu usage gets over this limit (0 = disable)
	CfgSpammerCPUMaxUsage = "spammer.cpuMaxUsage"
	// the rate limit for the spammer (0 = no limit)
	CfgSpammerMPSRateLimit = "spammer.mpsRateLimit"
	// the amount of parallel running spammers
	CfgSpammerWorkers = "spammer.workers"
	// CfgSpammerAutostart automatically starts the spammer on node startup
	CfgSpammerAutostart = "spammer.autostart"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.String(CfgSpammerMessage, "IOTA - A new dawn", "the message to embed within the spam messages")
		fs.String(CfgSpammerTag, "HORNET Spammer", "the tag of the message")
		fs.String(CfgSpammerTagSemiLazy, "HORNET Spammer Semi-Lazy", "the tag of the message if the semi-lazy pool is used (uses \"tag\" if empty)")
		fs.Float64(CfgSpammerCPUMaxUsage, 0.80, "workers remains idle for a while when cpu usage gets over this limit (0 = disable)")
		fs.Float64(CfgSpammerMPSRateLimit, 0.0, "the rate limit for the spammer (0 = no limit)")
		fs.Int(CfgSpammerWorkers, 0, "the amount of parallel running spammers")
		fs.Bool(CfgSpammerAutostart, false, "automatically start the spammer on node startup")
	},
	Masked: nil,
}
