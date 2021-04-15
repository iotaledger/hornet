package urts

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI is the maximum allowed delta
	// value for the YCRI of a given message in relation to the current CMI before it gets lazy.
	CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI = "tipsel.maxDeltaMsgYoungestConeRootIndexToCMI"
	// CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets semi-lazy.
	CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI = "tipsel.maxDeltaMsgOldestConeRootIndexToCMI"
	// CfgTipSelBelowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	CfgTipSelBelowMaxDepth = "tipsel.belowMaxDepth"
	// the config group used for the non-lazy tip-pool
	CfgTipSelNonLazy = "tipsel.nonLazy."
	// the config group used for the semi-lazy tip-pool
	CfgTipSelSemiLazy = "tipsel.semiLazy."
	// CfgTipSelRetentionRulesTipsLimit is the maximum amount of current tips for which "CfgTipSelMaxReferencedTipAge"
	// and "CfgTipSelMaxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network.
	CfgTipSelRetentionRulesTipsLimit = "retentionRulesTipsLimit"
	// CfgTipSelMaxReferencedTipAge is the maximum time a tip remains in the tip pool
	// after it was referenced by the first message.
	CfgTipSelMaxReferencedTipAge = "maxReferencedTipAge"
	// CfgTipSelMaxChildren is the maximum amount of references by other messages
	// before the tip is removed from the tip pool.
	CfgTipSelMaxChildren = "maxChildren"
	// CfgTipSelSpammerTipsThreshold is the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = disable (semi-lazy), 0 = always (non-lazy))
	// this is used to support the network if someone attacks the tangle by spamming a lot of tips
	CfgTipSelSpammerTipsThreshold = "spammerTipsThreshold"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Int(CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI, 8, "the maximum allowed delta "+
				"value for the YCRI of a given message in relation to the current CMI before it gets lazy")
			fs.Int(CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI, 13, "the maximum allowed delta "+
				"value between OCRI of a given message in relation to the current CMI before it gets semi-lazy")
			fs.Int(CfgTipSelBelowMaxDepth, 15, "the maximum allowed delta "+
				"value for the OCRI of a given message in relation to the current CMI before it gets lazy")
			fs.Int(CfgTipSelNonLazy+CfgTipSelRetentionRulesTipsLimit, 100, "the maximum number of current tips for which the retention rules are checked (non-lazy)")
			fs.Duration(CfgTipSelNonLazy+CfgTipSelMaxReferencedTipAge, 3*time.Second, "the maximum time a tip remains in the tip pool "+
				"after it was referenced by the first message (non-lazy)")
			fs.Int(CfgTipSelNonLazy+CfgTipSelMaxChildren, 30, "the maximum amount of references by other messages "+
				"before the tip is removed from the tip pool (non-lazy)")
			fs.Int(CfgTipSelNonLazy+CfgTipSelSpammerTipsThreshold, 0, "the maximum amount of tips in a tip-pool (non-lazy) before "+
				"the spammer tries to reduce these (0 = always)")
			fs.Int(CfgTipSelSemiLazy+CfgTipSelRetentionRulesTipsLimit, 20, "the maximum number of current tips for which the retention rules are checked (semi-lazy)")
			fs.Duration(CfgTipSelSemiLazy+CfgTipSelMaxReferencedTipAge, 3*time.Second, "the maximum time a tip remains in the tip pool "+
				"after it was referenced by the first message (semi-lazy)")
			fs.Int(CfgTipSelSemiLazy+CfgTipSelMaxChildren, 2, "the maximum amount of references by other messages "+
				"before the tip is removed from the tip pool (semi-lazy)")
			fs.Int(CfgTipSelSemiLazy+CfgTipSelSpammerTipsThreshold, 30, "the maximum amount of tips in a tip-pool (semi-lazy) before "+
				"the spammer tries to reduce these (0 = disable)")
			return fs
		}(),
	},
	Masked: nil,
}
