package spammer

import (
	"github.com/iotaledger/hive.go/app"
)

// ParametersSpammer contains the definition of the parameters used by WarpSync.
type ParametersSpammer struct {
	// the message to embed within the spam messages
	Message string `default:"We are all made of stardust." usage:"the message to embed within the spam messages"`
	// the tag of the message
	Tag string `default:"HORNET Spammer" usage:"the tag of the message"`
	// the tag of the message if the semi-lazy pool is used (uses "tag" if empty)
	TagSemiLazy string `default:"HORNET Spammer Semi-Lazy" usage:"the tag of the message if the semi-lazy pool is used (uses \"tag\" if empty)"`
	// workers remains idle for a while when cpu usage gets over this limit (0 = disable)
	CPUMaxUsage float64 `default:"0.80" usage:"workers remains idle for a while when cpu usage gets over this limit (0 = disable)"`
	// the rate limit for the spammer (0 = no limit)
	MPSRateLimit float64 `default:"0.0" usage:"the rate limit for the spammer (0 = no limit)"`
	// the amount of parallel running spammers
	Workers int `default:"0" usage:"the amount of parallel running spammers"`
	// whether to automatically start the spammer on node startup
	Autostart bool `default:"false" usage:"automatically start the spammer on node startup"`
}

var ParamsSpammer = &ParametersSpammer{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"spammer": ParamsSpammer,
	},
	Masked: nil,
}
