package urts

import (
	"time"

	"github.com/iotaledger/hive.go/app"
)

// ParametersTipsel contains the definition of the parameters used by Tipselection.
type ParametersTipsel struct {
	// the config group used for the non-lazy tip-pool
	NonLazy struct {
		// Defines the maximum amount of current tips for which "CfgTipSelMaxReferencedTipAge"
		// and "CfgTipSelMaxChildren" are checked. if the amount of tips exceeds this limit,
		// referenced tips get removed directly to reduce the amount of tips in the network.
		RetentionRulesTipsLimit int `default:"100" usage:"the maximum number of current tips for which the retention rules are checked (non-lazy)"`
		// Defines the maximum time a tip remains in the tip pool
		// after it was referenced by the first message.
		MaxReferencedTipAge time.Duration `default:"3s" usage:"the maximum time a tip remains in the tip pool after it was referenced by the first message (non-lazy)"`
		// Defines the maximum amount of references by other messages
		// before the tip is removed from the tip pool.
		MaxChildren uint32 `default:"30" usage:"the maximum amount of references by other messages before the tip is removed from the tip pool (non-lazy)"`
		// Defines the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = disable (semi-lazy), 0 = always (non-lazy))
		// this is used to support the network if someone attacks the tangle by spamming a lot of tips
		SpammerTipsThreshold int `default:"0" usage:"the maximum amount of tips in a tip-pool (non-lazy) before the spammer tries to reduce these (0 = always)"`
	}

	// the config group used for the semi-lazy tip-pool
	SemiLazy struct {
		// Defines the maximum amount of current tips for which "CfgTipSelMaxReferencedTipAge"
		// and "CfgTipSelMaxChildren" are checked. if the amount of tips exceeds this limit,
		// referenced tips get removed directly to reduce the amount of tips in the network.
		RetentionRulesTipsLimit int `default:"20" usage:"the maximum number of current tips for which the retention rules are checked (semi-lazy)"`
		// Defines the maximum time a tip remains in the tip pool
		// after it was referenced by the first message.
		MaxReferencedTipAge time.Duration `default:"3s" usage:"the maximum time a tip remains in the tip pool after it was referenced by the first message (semi-lazy)"`
		// Defines the maximum amount of references by other messages
		// before the tip is removed from the tip pool.
		MaxChildren uint32 `default:"2" usage:"the maximum amount of references by other messages before the tip is removed from the tip pool (semi-lazy)"`
		// Defines the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = disable (semi-lazy), 0 = always (non-lazy))
		// this is used to support the network if someone attacks the tangle by spamming a lot of tips
		SpammerTipsThreshold int `default:"30" usage:"the maximum amount of tips in a tip-pool (semi-lazy) before the spammer tries to reduce these (0 = disable)"`
	}
}

var ParamsTipsel = &ParametersTipsel{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"tipsel": ParamsTipsel,
	},
	Masked: nil,
}
