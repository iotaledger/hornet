package config

const (
	// CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI is the maximum allowed delta
	// value for the YTRSI of a given transaction in relation to the current LSMI before it gets lazy.
	CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI = "tipsel.maxDeltaTxYoungestRootSnapshotIndexToLSMI"
	// CfgTipSelMaxDeltaTxOldestRootSnapshotIndexToLSMI is the maximum allowed delta
	// value between OTRSI of a given transaction in relation to the current LSMI before it gets semi-lazy.
	CfgTipSelMaxDeltaTxOldestRootSnapshotIndexToLSMI = "tipsel.maxDeltaTxOldestRootSnapshotIndexToLSMI"
	// CfgTipSelBelowMaxDepth is the maximum allowed delta
	// value between OTRSI of a given transaction in relation to the current LSMI before it gets lazy.
	CfgTipSelBelowMaxDepth = "tipsel.belowMaxDepth"
	// the config group used for the non-lazy tip-pool
	CfgTipSelNonLazy = "tipsel.nonLazy."
	// the config group used for the semi-lazy tip-pool
	CfgTipSelSemiLazy = "tipsel.semiLazy."
	// CfgTipSelRetentionRulesTipsLimit is the maximum amount of current tips for which "CfgTipSelMaxReferencedTipAgeSeconds"
	// and "CfgTipSelMaxApprovers" are checked. if the amount of tips exceeds this limit,
	// referenced tips get removed directly to reduce the amount of tips in the network.
	CfgTipSelRetentionRulesTipsLimit = "retentionRulesTipsLimit"
	// CfgTipSelMaxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first transaction.
	CfgTipSelMaxReferencedTipAgeSeconds = "maxReferencedTipAgeSeconds"
	// CfgTipSelMaxApprovers is the maximum amount of references by other transactions
	// before the tip is removed from the tip pool.
	CfgTipSelMaxApprovers = "maxApprovers"
	// CfgTipSelSpammerTipsThreshold is the maximum amount of tips in a tip-pool before the spammer tries to reduce these (0 = disable (semi-lazy), 0 = always (non-lazy))
	// this is used to support the network if someone attacks the tangle by spamming a lot of tips
	CfgTipSelSpammerTipsThreshold = "spammerTipsThreshold"
)

func init() {
	configFlagSet.Int(CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI, 8, "the maximum allowed delta "+
		"value for the YTRSI of a given transaction in relation to the current LSMI before it gets lazy")
	configFlagSet.Int(CfgTipSelMaxDeltaTxOldestRootSnapshotIndexToLSMI, 13, "the maximum allowed delta "+
		"value between OTRSI of a given transaction in relation to the current LSMI before it gets semi-lazy")
	configFlagSet.Int(CfgTipSelBelowMaxDepth, 15, "the maximum allowed delta "+
		"value for the OTRSI of a given transaction in relation to the current LSMI before it gets lazy")
	configFlagSet.Int(CfgTipSelNonLazy+CfgTipSelRetentionRulesTipsLimit, 100, "the maximum number of current tips for which the retention rules are checked (non-lazy)")
	configFlagSet.Int(CfgTipSelNonLazy+CfgTipSelMaxReferencedTipAgeSeconds, 3, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first transaction (non-lazy)")
	configFlagSet.Int(CfgTipSelNonLazy+CfgTipSelMaxApprovers, 2, "the maximum amount of references by other transactions "+
		"before the tip is removed from the tip pool (non-lazy)")
	configFlagSet.Int(CfgTipSelNonLazy+CfgTipSelSpammerTipsThreshold, 0, "the maximum amount of tips in a tip-pool (non-lazy) before "+
		"the spammer tries to reduce these (0 = always)")
	configFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelRetentionRulesTipsLimit, 20, "the maximum number of current tips for which the retention rules are checked (semi-lazy)")
	configFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelMaxReferencedTipAgeSeconds, 3, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first transaction (semi-lazy)")
	configFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelMaxApprovers, 2, "the maximum amount of references by other transactions "+
		"before the tip is removed from the tip pool (semi-lazy)")
	configFlagSet.Int(CfgTipSelSemiLazy+CfgTipSelSpammerTipsThreshold, 30, "the maximum amount of tips in a tip-pool (semi-lazy) before "+
		"the spammer tries to reduce these (0 = disable)")
}
