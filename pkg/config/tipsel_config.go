package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI is the maximum allowed delta
	// value for the YTRSI of a given transaction in relation to the current LSMI.
	CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI = "tipsel.maxDeltaTxYoungestRootSnapshotIndexToLSMI"
	// CfgTipSelMaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI is the maximum allowed delta
	// value between OTRSI of the approvees of a given transaction in relation to the current LSMI.
	CfgTipSelMaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI = "tipsel.maxDeltaTxApproveesOldestRootSnapshotIndexToLSMI"
	// CfgTipSelBelowMaxDepth is a threshold value which indicates that a transaction
	// is not relevant in relation to the recent parts of the tangle.
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
)

func init() {
	flag.Int(CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI, 8, "the maximum allowed delta "+
		"value for the YTRSI of a given transaction in relation to the current LSMI")
	flag.Int(CfgTipSelMaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI, 13, "the maximum allowed delta "+
		"value between OTRSI of the approvees of a given transaction in relation to the current LSMI")
	flag.Int(CfgTipSelBelowMaxDepth, 15, "threshold value which indicates that a transaction "+
		"is not relevant in relation to the recent parts of the tangle")
	flag.Int(CfgTipSelNonLazy+CfgTipSelRetentionRulesTipsLimit, 100, "the maximum number of current tips for which the retention rules are checked (non-lazy)")
	flag.Int(CfgTipSelNonLazy+CfgTipSelMaxReferencedTipAgeSeconds, 3, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first transaction (non-lazy)")
	flag.Int(CfgTipSelNonLazy+CfgTipSelMaxApprovers, 2, "the maximum amount of references by other transactions "+
		"before the tip is removed from the tip pool (non-lazy)")
	flag.Int(CfgTipSelSemiLazy+CfgTipSelRetentionRulesTipsLimit, 20, "the maximum number of current tips for which the retention rules are checked (semi-lazy)")
	flag.Int(CfgTipSelSemiLazy+CfgTipSelMaxReferencedTipAgeSeconds, 3, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first transaction (semi-lazy)")
	flag.Int(CfgTipSelSemiLazy+CfgTipSelMaxApprovers, 2, "the maximum amount of references by other transactions "+
		"before the tip is removed from the tip pool (semi-lazy)")
}
