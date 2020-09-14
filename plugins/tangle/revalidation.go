package tangle

import (
	"errors"
	"time"

	"github.com/iotaledger/hive.go/daemon"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	// printStatusInterval is the interval for printing status messages
	printStatusInterval = 2 * time.Second
)

var (
	ErrSnapshotInfoMissing                   = errors.New("snapshot information not found in database")
	ErrLatestMilestoneOlderThanSnapshotIndex = errors.New("latest milestone in the database is older than the snapshot index")
	ErrSnapshotIndexWrong                    = errors.New("snapshot index does not fit the snapshot ledger index")
)

// revalidateDatabase tries to revalidate a corrupted database (after an unclean node shutdown/crash)
//
// HORNET uses caches for almost all tangle related data.
// If the node crashes, it is not guaranteed that all data in the cache was already persisted to the disk.
// Thats why we flag the database as corrupted.
//
// This function tries to restore a clean database state by deleting all existing transactions
// since last local snapshot, deleting all ledger states and changes, loading valid snapshot ledger state.
//
// This way HORNET should be able to re-solidify the existing tangle in the database.
//
// Object Storages:
// 		Stored with caching:
//			- TxRaw (synced)					=> will be removed and added again by requesting the tx at solidification
//			- TxMetadata (synced)				=> will be removed and added again if missing by receiving the tx (if not => reset)
//			- BundleTransaction (synced)		=> will be removed and added again if missing by receiving the tx
//			- Bundle (always)					=> will be removed and added again if missing by receiving the tx
//			- Approver (synced)					=> will be removed and added again if missing by receiving the tx
//
// 		Stored without caching:
//			- Tag								=> will be removed and added again if missing by receiving the tx
//			- Address							=> will be removed and added again if missing by receiving the tx
//			- UnconfirmedTx 					=> will be removed at pruning anyway
//			- Milestone							=> will be removed and added again by receiving the tx
//			- SpentAddresses					=> will be removed and added again if missing by receiving the tx
//
// Database:
// 		- LedgerState
//			- Balances of latest solid milestone		=> will be removed and replaced with snapshot milestone
//			- Balances of snapshot milestone			=> should be consistent (total iotas are checked)
//			- Balance diffs of every solid milestone	=> will be removed and added again by confirmation
//
func revalidateDatabase() error {

	// mark the database as tainted forever.
	// this is used to signal the coordinator plugin that it should never use a revalidated database.
	tangle.MarkDatabaseTainted()

	start := time.Now()

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		return ErrSnapshotInfoMissing
	}

	latestMilestoneIndex := tangle.SearchLatestMilestoneIndexInStore()

	if snapshotInfo.SnapshotIndex > latestMilestoneIndex && (latestMilestoneIndex != 0) {
		return ErrLatestMilestoneOlderThanSnapshotIndex
	}

	log.Infof("reverting database state back from %d to local snapshot %d (this might take a while)... ", latestMilestoneIndex, snapshotInfo.SnapshotIndex)

	// delete milestone data newer than the local snapshot
	if err := cleanupMilestones(snapshotInfo); err != nil {
		return err
	}

	// deletes all ledger diffs which have a confirmation milestone newer than the last local snapshot's milestone.
	if err := cleanupLedgerDiffs(snapshotInfo); err != nil {
		return err
	}

	// clean up transactions which are above the local snapshot
	if err := cleanupTransactions(snapshotInfo); err != nil {
		return err
	}

	// deletes all transaction metadata where the tx doesn't exist in the database anymore.
	if err := cleanupTransactionMetadata(); err != nil {
		return err
	}

	// deletes all bundles where a single tx of the bundle doesn't exist in the database anymore.
	if err := cleanupBundles(); err != nil {
		return err
	}

	// deletes all bundles transactions where the tx doesn't exist in the database anymore.
	if err := cleanupBundleTransactions(); err != nil {
		return err
	}

	// deletes all approvers where the tx doesn't exist in the database anymore.
	if err := cleanupApprovers(); err != nil {
		return err
	}

	// deletes all tags where the tx doesn't exist in the database anymore.
	if err := cleanupTags(); err != nil {
		return err
	}

	// deletes all addresses where the tx doesn't exist in the database anymore.
	if err := cleanupAddresses(); err != nil {
		return err
	}

	// deletes all unconfirmed txs that are left in the database (we do not need them since we deleted all unconfirmed txs).
	if err := cleanupUnconfirmedTxs(); err != nil {
		return err
	}

	log.Info("flushing storages...")
	tangle.FlushStorages()
	log.Info("flushing storages... done!")

	// apply the ledger from the last snapshot to the database
	if err := applySnapshotLedger(snapshotInfo); err != nil {
		return err
	}

	log.Infof("reverted state back to local snapshot %d, took %v", snapshotInfo.SnapshotIndex, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes milestones above the given snapshot's milestone index.
func cleanupMilestones(info *tangle.SnapshotInfo) error {

	start := time.Now()

	milestonesToDelete := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var milestonesCounter int64
	tangle.ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		milestonesCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d milestones", milestonesCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex {
			return true
		}

		milestonesToDelete[msIndex] = struct{}{}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(milestonesToDelete)
	var deletionCounter int64
	for msIndex := range milestonesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting milestones...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteUnconfirmedTxs(msIndex)
		if err := tangle.DeleteLedgerDiffForMilestone(msIndex); err != nil {
			panic(err)
		}

		tangle.DeleteMilestone(msIndex)
	}

	tangle.FlushUnconfirmedTxsStorage()
	tangle.FlushMilestoneStorage()

	log.Infof("deleting milestones...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all ledger diffs which have a confirmation milestone newer than the last local snapshot's milestone.
func cleanupLedgerDiffs(info *tangle.SnapshotInfo) error {

	start := time.Now()

	ledgerDiffsToDelete := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var ledgerDiffsCounter int64
	tangle.ForEachLedgerDiffHash(func(msIndex milestone.Index, address hornet.Hash) bool {
		ledgerDiffsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d ledger diffs", ledgerDiffsCounter)
		}

		// do not delete older milestones
		if msIndex <= info.SnapshotIndex {
			return true
		}

		ledgerDiffsToDelete[msIndex] = struct{}{}

		return true
	}, true)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(ledgerDiffsToDelete)
	var deletionCounter int64
	for msIndex := range ledgerDiffsToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting ledger diffs...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteLedgerDiffForMilestone(msIndex)
	}

	log.Infof("deleting ledger diffs...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all transactions which are not confirmed, not solid or
// their confirmation milestone is newer than the last local snapshot's milestone.
func cleanupTransactions(info *tangle.SnapshotInfo) error {

	start := time.Now()

	transactionsToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var txsCounter int64
	tangle.ForEachTransactionHash(func(txHash hornet.Hash) bool {
		txsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d transactions", txsCounter)
		}

		storedTxMeta := tangle.GetStoredMetadataOrNil(txHash)

		// delete transaction if metadata doesn't exist
		if storedTxMeta == nil {
			transactionsToDelete[string(txHash)] = struct{}{}
			return true
		}

		// not solid
		if !storedTxMeta.IsSolid() {
			transactionsToDelete[string(txHash)] = struct{}{}
			return true
		}

		// not confirmed or above snapshot index
		if confirmed, by := storedTxMeta.GetConfirmed(); !confirmed || by > info.SnapshotIndex {
			transactionsToDelete[string(txHash)] = struct{}{}
			return true
		}

		return true
	}, true)
	log.Infof("analyzed %d transactions", txsCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(transactionsToDelete)
	var deletionCounter int64
	for txHash := range transactionsToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting transactions...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteTransaction(hornet.Hash(txHash))
	}

	tangle.FlushTransactionStorage()

	log.Infof("deleting transactions...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all transaction metadata where the tx doesn't exist in the database anymore.
func cleanupTransactionMetadata() error {

	start := time.Now()

	metadataToDelete := make(map[string]struct{})

	lastStatusTime := time.Now()
	var metadataCounter int64
	tangle.ForEachTransactionMetadataHash(func(txHash hornet.Hash) bool {
		metadataCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d transaction metadata", metadataCounter)
		}

		// delete metadata if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			metadataToDelete[string(txHash)] = struct{}{}
		}

		return true
	}, true)
	log.Infof("analyzed %d transaction metadata", metadataCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(metadataToDelete)
	var deletionCounter int64
	for txHash := range metadataToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting transaction metadata...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteTransactionMetadata(hornet.Hash(txHash))
	}

	tangle.FlushTransactionStorage()

	log.Infof("deleting transaction metadata...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all bundles where a single tx of the bundle doesn't exist in the database anymore.
func cleanupBundles() error {

	start := time.Now()

	bundlesToDelete := make(map[string]struct{})

	var bundleCounter int64
	lastStatusTime := time.Now()
	tangle.ForEachBundleHash(func(tailTxHash hornet.Hash) bool {
		bundleCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d bundles", bundleCounter)
		}

		bundle := tangle.GetStoredBundleOrNil(tailTxHash)
		if bundle == nil {
			return true
		}

		for _, txHash := range bundle.GetTxHashes() {
			// delete bundle if a single transaction of the bundle doesn't exist
			if !tangle.TransactionExistsInStore(txHash) {
				bundlesToDelete[string(tailTxHash)] = struct{}{}
				return true
			}
		}

		return true
	}, true)
	log.Infof("analyzed %d bundles", bundleCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(bundlesToDelete)
	var deletionCounter int64
	for tailTxHash := range bundlesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting bundles...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteBundle(hornet.Hash(tailTxHash))
	}

	tangle.FlushBundleStorage()

	log.Infof("deleting bundles...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all bundles transactions where the tx doesn't exist in the database anymore.
func cleanupBundleTransactions() error {

	type bundleTransaction struct {
		bundleHash hornet.Hash
		isTail     bool
		txHash     hornet.Hash
	}

	start := time.Now()

	bundleTxsToDelete := make(map[string]*bundleTransaction)

	lastStatusTime := time.Now()
	var bundleTxsCounter int64
	tangle.ForEachBundleTransaction(func(bundleHash hornet.Hash, txHash hornet.Hash, isTail bool) bool {
		bundleTxsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d bundle transactions", bundleTxsCounter)
		}

		// delete bundle transaction if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			bundleTxsToDelete[string(txHash)] = &bundleTransaction{bundleHash: bundleHash, txHash: txHash, isTail: isTail}
		}

		return true
	}, true)
	log.Infof("analyzed %d bundle transactions", bundleTxsCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(bundleTxsToDelete)
	var deletionCounter int64
	for _, bundleTx := range bundleTxsToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting bundle transactions...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteBundleTransaction(bundleTx.bundleHash, bundleTx.txHash, bundleTx.isTail)
	}

	tangle.FlushBundleTransactionsStorage()

	log.Infof("deleting bundle transactions...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all approvers where the tx doesn't exist in the database anymore.
func cleanupApprovers() error {

	type approver struct {
		txHash       hornet.Hash
		approverHash hornet.Hash
	}

	start := time.Now()

	approversToDelete := make(map[string]*approver)

	lastStatusTime := time.Now()
	var approverCounter int64
	tangle.ForEachApprover(func(txHash hornet.Hash, approverHash hornet.Hash) bool {
		approverCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d approvers", approverCounter)
		}

		// delete approver if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			approversToDelete[string(txHash)+string(approverHash)] = &approver{txHash: txHash, approverHash: approverHash}
		}

		// delete approver if approver transaction doesn't exist
		if !tangle.TransactionExistsInStore(approverHash) {
			approversToDelete[string(txHash)+string(approverHash)] = &approver{txHash: txHash, approverHash: approverHash}
		}

		return true
	}, true)
	log.Infof("analyzed %d approvers", approverCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(approversToDelete)
	var deletionCounter int64
	for _, approver := range approversToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting approvers...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteApprover(approver.txHash, approver.approverHash)
	}

	tangle.FlushApproversStorage()

	log.Infof("deleting approvers...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all tags where the tx doesn't exist in the database anymore.
func cleanupTags() error {

	type tag struct {
		tag    hornet.Hash
		txHash hornet.Hash
	}

	start := time.Now()

	tagsToDelete := make(map[string]*tag)

	lastStatusTime := time.Now()
	var tagsCounter int64
	tangle.ForEachTag(func(txTag hornet.Hash, txHash hornet.Hash) bool {
		tagsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d tags", tagsCounter)
		}

		// delete tag if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			tagsToDelete[string(txHash)] = &tag{tag: txTag, txHash: txHash}
		}

		return true
	}, true)
	log.Infof("analyzed %d tags", tagsCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(tagsToDelete)
	var deletionCounter int64
	for _, tag := range tagsToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting tags...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteTag(tag.tag, tag.txHash)
	}

	tangle.FlushTagsStorage()

	log.Infof("deleting tags...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all addresses where the tx doesn't exist in the database anymore.
func cleanupAddresses() error {

	type address struct {
		address hornet.Hash
		txHash  hornet.Hash
	}

	addressesToDelete := make(map[string]*address)

	start := time.Now()

	lastStatusTime := time.Now()
	var addressesCounter int64
	tangle.ForEachAddress(func(addressHash hornet.Hash, txHash hornet.Hash, isValue bool) bool {
		addressesCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d addresses", addressesCounter)
		}

		// delete address if transaction doesn't exist
		if !tangle.TransactionExistsInStore(txHash) {
			addressesToDelete[string(txHash)] = &address{address: addressHash, txHash: txHash}
		}

		return true
	}, true)
	log.Infof("analyzed %d addresses", addressesCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(addressesToDelete)
	var deletionCounter int64
	for _, addr := range addressesToDelete {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting addresses...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteAddress(addr.address, addr.txHash)
	}

	tangle.FlushAddressStorage()

	log.Infof("deleting addresses...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// deletes all unconfirmed txs that are left in the database (we do not need them since we deleted all unconfirmed txs).
func cleanupUnconfirmedTxs() error {

	start := time.Now()

	unconfirmedMilestoneIndexes := make(map[milestone.Index]struct{})

	lastStatusTime := time.Now()
	var unconfirmedTxsCounter int64
	tangle.ForEachUnconfirmedTx(func(msIndex milestone.Index, txHash hornet.Hash) bool {
		unconfirmedTxsCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return false
			}

			log.Infof("analyzed %d unconfirmed txs", unconfirmedTxsCounter)
		}

		unconfirmedMilestoneIndexes[msIndex] = struct{}{}

		return true
	}, true)
	log.Infof("analyzed %d unconfirmed txs", unconfirmedTxsCounter)

	if daemon.IsStopped() {
		return tangle.ErrOperationAborted
	}

	total := len(unconfirmedMilestoneIndexes)
	var deletionCounter int64
	for msIndex := range unconfirmedMilestoneIndexes {
		deletionCounter++

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			if daemon.IsStopped() {
				return tangle.ErrOperationAborted
			}

			percentage, remaining := utils.EstimateRemainingTime(start, deletionCounter, int64(total))
			log.Infof("deleting unconfirmed txs...%d/%d (%0.2f%%). %v left...", deletionCounter, total, percentage, remaining.Truncate(time.Second))
		}

		tangle.DeleteUnconfirmedTxs(msIndex)
	}

	tangle.FlushUnconfirmedTxsStorage()

	log.Infof("deleting unconfirmed txs...%d/%d (100.00%%) done. took %v", total, total, time.Since(start).Truncate(time.Millisecond))

	return nil
}

// apply the ledger from the last snapshot to the database
func applySnapshotLedger(info *tangle.SnapshotInfo) error {

	log.Info("applying snapshot balances to the ledger state...")
	// Get the ledger state of the last snapshot
	snapshotBalances, snapshotIndex, err := tangle.GetAllSnapshotBalances(nil)
	if err != nil {
		return err
	}

	if info.SnapshotIndex != snapshotIndex {
		return ErrSnapshotIndexWrong
	}

	// Store the snapshot balances as the current valid ledger
	if err = tangle.StoreLedgerBalancesInDatabase(snapshotBalances, snapshotIndex); err != nil {
		return err
	}
	log.Info("applying snapshot balances to the ledger state ... done!")

	// Set the valid solid milestone index
	tangle.OverwriteSolidMilestoneIndex(info.SnapshotIndex)

	return nil
}
