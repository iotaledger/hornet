package urts

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI is the maximum allowed delta
	// value for the YCRI of a given message in relation to the current LSMI before it gets lazy.
	CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI = "tipsel.maxDeltaMsgYoungestConeRootIndexToLSMI"
	// CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI is the maximum allowed delta
	// value between OCRI of a given message in relation to the current LSMI before it gets semi-lazy.
	CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI = "tipsel.maxDeltaMsgOldestConeRootIndexToLSMI"
	// CfgTipSelBelowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current LSMI before it gets lazy.
	CfgTipSelBelowMaxDepth = "tipsel.belowMaxDepth"
	// the config group used for the non-lazy tip-pool
	CfgTipSelNonLazy = "tipsel.nonLazy."
	// the config group used for the semi-lazy tip-pool
	CfgTipSelSemiLazy = "tipsel.semiLazy."
	// CfgTipSelRetentionRulesTipsLimit is the maximum amount of current tips for which "CfgTipSelMaxReferencedTipAgeSeconds"
	// and "CfgTipSelMaxChildren" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network.
	CfgTipSelRetentionRulesTipsLimit = "retentionRulesTipsLimit"
	// CfgTipSelMaxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first message.
	CfgTipSelMaxReferencedTipAgeSeconds = "maxReferencedTipAgeSeconds"
	// CfgTipSelMaxChildren is the maximum amount of references by other messages
	// before the tip is removed from the tip pool.
	CfgTipSelMaxChildren = "maxChildren"
	// CfgTipSelSpammerTipsThreshold is the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = disable (semi-lazy), 0 = always (non-lazy))
	// this is used to support the network if someone attacks the tangle by spamming a lot of tips
	CfgTipSelSpammerTipsThreshold = "spammerTipsThreshold"
)

func init() {
	cli.ConfigFlagSet.Int(CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI, 8, "the maximum allowed delta "+
		"value for the YCRI of a given message in relation to the current LSMI before it gets lazy")
	cli.ConfigFlagSet.Int(CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI, 13, "the maximum allowed delta "+
		"value between OCRI of a given message in relation to the current LSMI before it gets semi-lazy")
	cli.ConfigFlagSet.Int(CfgTipSelBelowMaxDepth, 15, "the maximum allowed delta "+
		"value for the OCRI of a given message in relation to the current LSMI before it gets lazy")
	cli.ConfigFlagSet.Int(CfgTipSelNonLazy+CfgTipSelRetentionRulesTipsLimit, 100, "the maximum number of current tips for which the retention rules are checked (non-lazy)")
	cli.ConfigFlagSet.Int(CfgTipSelNonLazy+CfgTipSelMaxReferencedTipAgeSeconds, 3, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first message (non-lazy)")
	cli.ConfigFlagSet.Int(CfgTipSelNonLazy+CfgTipSelMaxChildren, 2, "the maximum amount of references by other messages "+
		"before the tip is removed from the tip pool (non-lazy)")
	cli.ConfigFlagSet.Int(CfgTipSelNonLazy+CfgTipSelSpammerTipsThreshold, 0, "the maximum amount of tips in a tip-pool (non-lazy) before "+
		"the spammer tries to reduce these (0 = always)")
	cli.ConfigFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelRetentionRulesTipsLimit, 20, "the maximum number of current tips for which the retention rules are checked (semi-lazy)")
	cli.ConfigFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelMaxReferencedTipAgeSeconds, 3, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first message (semi-lazy)")
	cli.ConfigFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelMaxChildren, 2, "the maximum amount of references by other messages "+
		"before the tip is removed from the tip pool (semi-lazy)")
	cli.ConfigFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelSpammerTipsThreshold, 30, "the maximum amount of tips in a tip-pool (semi-lazy) before "+
		"the spammer tries to reduce these (0 = disable)")
}
