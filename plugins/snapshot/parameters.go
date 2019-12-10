package snapshot

import flag "github.com/spf13/pflag"

func init() {
	flag.Bool("localSnapshots.enabled", true, "Enable local snapshots")
	flag.Bool("localSnapshots.pruningEnabled", true, "Enable your node to delete old transactions from its database")
	flag.Int("localSnapshots.depth", 100, "Amount of seen milestones to record in the snapshot file")
	flag.Int("localSnapshots.pruningDelay", 40000, "Amount of milestone transactions to keep in the ledger")
	flag.Int("localSnapshots.intervalSynced", 10, "Interval, in milestone transactions, at which snapshot files are created if the ledger is fully synchronized")
	flag.Int("localSnapshots.intervalUnsynced", 1000, "Interval, in milestone transactions, at which snapshot files are created if the ledger is not fully synchronized")
	flag.String("localSnapshots.path", "latest-export.gz.bin", "Path to the snapshot file")
}
