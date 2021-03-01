package spammer

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

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

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgSpammerMessage, "Binary is the future.", "the message to embed within the spam messages")
			fs.String(CfgSpammerIndex, "HORNET Spammer", "the indexation of the message")
			fs.String(CfgSpammerIndexSemiLazy, "HORNET Spammer Semi-Lazy", "the indexation of the message if the semi-lazy pool is used (uses \"index\" if empty)")
			fs.Float64(CfgSpammerCPUMaxUsage, 0.50, "workers remains idle for a while when cpu usage gets over this limit (0 = disable)")
			fs.Float64(CfgSpammerMPSRateLimit, 0.0, "the rate limit for the spammer (0 = no limit)")
			fs.Int(CfgSpammerWorkers, 1, "the amount of parallel running spammers")
			fs.Bool(CfgSpammerAutostart, false, "automatically start the spammer on node startup")
			return fs
		}(),
	},
	Masked: nil,
}
