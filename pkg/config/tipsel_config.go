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
	// CfgTipSelMaxReferencedTipAgeSeconds is the maximum time a tip remains in the tip pool
	// after it was referenced by the first transaction.
	CfgTipSelMaxReferencedTipAgeSeconds = "tipsel.maxReferencedTipAgeSeconds"
	// CfgTipSelMaxApprovers is the maximum amount of references by other transactions
	// before the tip is removed from the tip pool.
	CfgTipSelMaxApprovers = "tipsel.maxApprovers"
)

func init() {
	flag.Int(CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI, 2, "the maximum allowed delta "+
		"value for the YTRSI of a given transaction in relation to the current LSMI")
	flag.Int(CfgTipSelMaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI, 7, "the maximum allowed delta "+
		"value between OTRSI of the approvees of a given transaction in relation to the current LSMI")
	flag.Int(CfgTipSelBelowMaxDepth, 15, "threshold value which indicates that a transaction "+
		"is not relevant in relation to the recent parts of the tangle")
	flag.Int(CfgTipSelMaxReferencedTipAgeSeconds, 30, "the maximum time a tip remains in the tip pool "+
		"after it was referenced by the first transaction")
	flag.Int(CfgTipSelMaxApprovers, 2, "the maximum amount of references by other transactions "+
		"before the tip is removed from the tip pool")
}
