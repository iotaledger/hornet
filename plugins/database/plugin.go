package database

import (
	"bytes"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
)

var (
	PLUGIN = node.NewPlugin("Database", node.Enabled, configure, run)
	log    *logger.Logger

	garbageCollectionLock syncutils.Mutex
)

// pruneTransactions prunes the approvers, bundles, bundle txs, addresses, tags and transaction metadata from the database
func pruneTransactions(txsToCheckMap map[string]struct{}) int {

	txsToDeleteMap := make(map[string]struct{})

	for txHashToCheck := range txsToCheckMap {

		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(txHashToCheck)) // tx +1
		if cachedTxMeta == nil {
			log.Warnf("pruneTransactions: Transaction not found: %s", txHashToCheck)
			continue
		}

		for txToRemove := range tangle.RemoveTransactionFromBundle(cachedTxMeta.GetMetadata()) {
			txsToDeleteMap[txToRemove] = struct{}{}
		}
		// since it gets loaded below again it doesn't make sense to force release here
		cachedTxMeta.Release() // tx -1
	}

	for txHashToDelete := range txsToDeleteMap {

		cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHashToDelete)) // tx +1
		if cachedTx == nil {
			continue
		}

		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction) { // tx -1
			// Delete the reference in the approvees
			tangle.DeleteApprover(tx.GetTrunkHash(), tx.GetTxHash())
			tangle.DeleteApprover(tx.GetBranchHash(), tx.GetTxHash())

			tangle.DeleteTag(tx.GetTag(), tx.GetTxHash())
			tangle.DeleteAddress(tx.GetAddress(), tx.GetTxHash())
			tangle.DeleteApprovers(tx.GetTxHash())
			tangle.DeleteTransaction(tx.GetTxHash())
		})
	}

	return len(txsToDeleteMap)
}

// pruneMilestone prunes the milestone metadata and the ledger diffs from the database for the given milestone
func pruneMilestone(milestoneIndex milestone.Index) {

	// state diffs
	if err := tangle.DeleteLedgerDiffForMilestone(milestoneIndex); err != nil {
		log.Warn(err)
	}

	tangle.DeleteMilestone(milestoneIndex)
}

func deleteInvalidMilestones() {

	invalidMilestoneBundles := []hornet.Hash{
		hornet.HashFromHashTrytes("JVYIDMDDFISYRAMQL9W9YQTDSWKVOLHOISWTFDVNNQGZAQUYLWMYYMLXQQCVVVXJWHWG9ULP9QEJJUQDC"),
		hornet.HashFromHashTrytes("RKAZMTNNOOZGEIS9SYUVLFRRRRFWALTHSSKZPVNQPSCIX9CYHTIJXNWBXVQECJYLJKY99YMWIBBJXILXC"),
	}

	txsToCheck := map[string]struct{}{}

	invalidMilestonesIndexes := []milestone.Index{2272660, 2272661}
	for _, invalidMilestonesIndex := range invalidMilestonesIndexes {
		cachedMs := tangle.GetMilestoneOrNil(invalidMilestonesIndex)
		if cachedMs == nil {
			continue
		}

		for _, bundleHash := range invalidMilestoneBundles {
			if !bytes.Equal(cachedMs.GetBundle().GetBundleHash(), bundleHash) {
				continue
			}

			// invalid milestone detected
			for _, txHash := range cachedMs.GetBundle().GetTxHashes() {
				txsToCheck[string(txHash)] = struct{}{}
			}

			log.Warnf("Deleting invalid milestone %d (%s)", invalidMilestonesIndex, bundleHash.Trytes())
			pruneMilestone(invalidMilestonesIndex)
		}

		cachedMs.Release(true)
	}

	pruneTransactions(txsToCheck)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	tangle.ConfigureDatabases(config.NodeConfig.GetString(config.CfgDatabasePath))

	deleteInvalidMilestones()

	if !tangle.IsCorrectDatabaseVersion() {
		if !tangle.UpdateDatabaseVersion() {
			log.Panic("HORNET database version mismatch. The database scheme was updated. Please delete the database folder and start with a new local snapshot.")
		}
	}

	daemon.BackgroundWorker("Close database", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		tangle.MarkDatabaseHealthy()
		log.Info("Syncing databases to disk...")
		tangle.CloseDatabases()
		log.Info("Syncing databases to disk... done")
	}, shutdown.PriorityCloseDatabase)
}

func RunGarbageCollection() {
	if tangle.DatabaseSupportsCleanup() {

		garbageCollectionLock.Lock()
		defer garbageCollectionLock.Unlock()

		log.Info("running full database garbage collection. This can take a while...")

		start := time.Now()

		Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
			Start: start,
		})

		err := tangle.CleanupDatabases()

		end := time.Now()

		Events.DatabaseCleanup.Trigger(&DatabaseCleanup{
			Start: start,
			End:   end,
		})

		if err != nil {
			if err != tangle.ErrNothingToCleanUp {
				log.Warnf("full database garbage collection failed with error: %s. took: %v", err.Error(), end.Sub(start).Truncate(time.Millisecond))
				return
			}
		}

		log.Infof("full database garbage collection finished. took %v", end.Sub(start).Truncate(time.Millisecond))
	}
}

func run(_ *node.Plugin) {
	// do nothing
}
