package snapshot

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/dag"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

const (
	SpentAddressesImportBatchSize       = 100000
	SolidEntryPointCheckThresholdPast   = 50
	SolidEntryPointCheckThresholdFuture = 50
)

var (
	SupportedLocalSnapshotFileVersions = []byte{4}
)

var ErrUnsupportedLSFileVersion = errors.New("unsupported local snapshot file version")

// isSolidEntryPoint checks whether any direct approver of the given transaction was confirmed by a milestone which is above the target milestone.
func isSolidEntryPoint(txHash trinary.Hash, targetIndex milestone_index.MilestoneIndex) (bool, milestone_index.MilestoneIndex) {

	for _, approverHash := range tangle.GetApproverHashes(txHash, true) {
		cachedTx := tangle.GetCachedTransactionOrNil(approverHash) // tx +1
		if cachedTx == nil {
			log.Panicf("isSolidEntryPoint: Transaction not found: %v", approverHash)
		}

		// HINT: Check for orphaned Tx as solid entry points is skipped in HORNET, since this operation is heavy and not necessary, and
		//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
		//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old tx as valid tips, which is not the case anymore.

		confirmed, at := cachedTx.GetMetadata().GetConfirmed()
		cachedTx.Release(true) // tx -1
		if confirmed && (at > targetIndex) {
			// confirmed by a later milestone than targetIndex => solidEntryPoint

			return true, at
		}
	}

	return false, 0
}

// getMilestoneApprovees traverses a milestone and collects all tx that were confirmed by that milestone or higher
func getMilestoneApprovees(milestoneIndex milestone_index.MilestoneIndex, cachedMsTailTx *tangle.CachedTransaction, collectForPruning bool, abortSignal <-chan struct{}) ([]trinary.Hash, error) {

	defer cachedMsTailTx.Release(true) // tx -1

	ts := time.Now()

	txsToTraverse := make(map[trinary.Hash]struct{})
	txsChecked := make(map[trinary.Hash]struct{})
	var approvees []trinary.Hash
	txsToTraverse[cachedMsTailTx.GetTransaction().GetHash()] = struct{}{}

	// Collect all tx by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			select {
			case <-abortSignal:
				return nil, ErrSnapshotCreationWasAborted
			default:
			}

			if _, checked := txsChecked[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}
			txsChecked[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				if !collectForPruning {
					log.Panicf("getMilestoneApprovees: Transaction not found: %v", txHash)
				}

				// Go on if the tx is missing (needed for pruning of the database)
				continue
			}

			if confirmed, at := cachedTx.GetMetadata().GetConfirmed(); confirmed {
				if at < milestoneIndex {
					// Ignore Tx that were confirmed by older milestones
					cachedTx.Release(true) // tx -1
					continue
				}
			} else {
				// Tx is not confirmed
				// ToDo: This shouldn't happen, but it does since tipselection allows it at the moment
				if !collectForPruning {
					if cachedTx.GetTransaction().IsTail() {
						cachedTx.Release(true) // tx -1
						log.Panicf("getMilestoneApprovees: Transaction not confirmed: %v", txHash)
					}

					// Search all referenced tails of this Tx (needed for correct SolidEntryPoint calculation).
					// This non-tail tx was not confirmed by the milestone, and could be referenced by the future cone.
					// Thats why we have to search all tail txs that get referenced by this incomplete bundle, to mark them as SEPs.
					tailTxs, err := dag.FindAllTails(txHash, true)
					if err != nil {
						cachedTx.Release(true) // tx -1
						return nil, err
					}

					for tailTx := range tailTxs {
						txsToTraverse[tailTx] = struct{}{}
					}

					// Ignore this transaction in the cone because it is not confirmed
					cachedTx.Release(true) // tx -1
					continue
				}

				// Check if the we can walk further => if not, it should be fine (only used for pruning anyway)
				if !tangle.ContainsTransaction(cachedTx.GetTransaction().GetTrunk()) {
					// Do not force release, since it is loaded again
					cachedTx.Release() // tx -1
					approvees = append(approvees, txHash)
					continue
				}

				if !tangle.ContainsTransaction(cachedTx.GetTransaction().GetBranch()) {
					// Do not force release, since it is loaded again
					cachedTx.Release() // tx -1
					approvees = append(approvees, txHash)
					continue
				}
			}

			approvees = append(approvees, txHash)

			// Traverse the approvee
			txsToTraverse[cachedTx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[cachedTx.GetTransaction().GetBranch()] = struct{}{}

			// Do not force release, since it is loaded again
			cachedTx.Release() // tx -1
		}
	}

	log.Debugf("Milestone walked (%d): approvees: %v, collect: %v", milestoneIndex, len(approvees), time.Since(ts))
	return approvees, nil
}

func shouldTakeSnapshot(solidMilestoneIndex milestone_index.MilestoneIndex) bool {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	var snapshotInterval milestone_index.MilestoneIndex
	if tangle.IsNodeSynced() {
		snapshotInterval = snapshotIntervalSynced
	} else {
		snapshotInterval = snapshotIntervalUnsynced
	}

	if (solidMilestoneIndex - snapshotDepth) < snapshotInfo.PruningIndex+1+SolidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return solidMilestoneIndex-(snapshotDepth+snapshotInterval) >= snapshotInfo.SnapshotIndex
}

func getSolidEntryPoints(targetIndex milestone_index.MilestoneIndex, abortSignal <-chan struct{}) (map[string]milestone_index.MilestoneIndex, error) {

	solidEntryPoints := make(map[string]milestone_index.MilestoneIndex)
	solidEntryPoints[consts.NullHashTrytes] = targetIndex

	// HINT: Check if "old solid entry points are still valid" is skipped in HORNET,
	//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
	//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old tx as valid tips, which is not the case anymore.

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - SolidEntryPointCheckThresholdPast; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			return nil, ErrSnapshotCreationWasAborted
		default:
		}

		cachedMs := tangle.GetMilestoneOrNil(milestoneIndex) // bundle +1
		if cachedMs == nil {
			log.Panicf("CreateLocalSnapshot: Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		cachedMsTailTx := cachedMs.GetBundle().GetTail() // tx +1
		cachedMs.Release(true)                           // bundle -1

		approvees, err := getMilestoneApprovees(milestoneIndex, cachedMsTailTx.Retain(), false, abortSignal)

		// Do not force release, since it is loaded again
		cachedMsTailTx.Release() // tx -1

		if err != nil {
			return nil, err
		}

		for _, approvee := range approvees {
			select {
			case <-abortSignal:
				return nil, ErrSnapshotCreationWasAborted
			default:
			}

			if isEntryPoint, at := isSolidEntryPoint(approvee, targetIndex); isEntryPoint {
				// A solid entry point should only be a tail transaction, otherwise the whole bundle can't be reproduced with a snapshot file
				tails, err := dag.FindAllTails(approvee, true)
				if err != nil {
					log.Panicf("CreateLocalSnapshot: %v", err)
				}

				for tailHash := range tails {
					solidEntryPoints[tailHash] = at
				}
			}
		}
	}

	return solidEntryPoints, nil
}

func getSeenMilestones(targetIndex milestone_index.MilestoneIndex, abortSignal <-chan struct{}) (map[string]milestone_index.MilestoneIndex, error) {

	// Fill the list with seen milestones
	seenMilestones := make(map[string]milestone_index.MilestoneIndex)
	lastestMilestone := tangle.GetLatestMilestoneIndex()
	for milestoneIndex := targetIndex + 1; milestoneIndex <= lastestMilestone; milestoneIndex++ {
		select {
		case <-abortSignal:
			return nil, ErrSnapshotCreationWasAborted
		default:
		}

		cachedMs := tangle.GetMilestoneOrNil(milestoneIndex) // bundle +1
		if cachedMs == nil {
			continue
		}
		seenMilestones[cachedMs.GetBundle().GetTailHash()] = milestoneIndex
		cachedMs.Release(true) // bundle -1
	}
	return seenMilestones, nil
}

func getLedgerStateAtMilestone(balances map[trinary.Hash]uint64, targetIndex milestone_index.MilestoneIndex, solidMilestoneIndex milestone_index.MilestoneIndex, abortSignal <-chan struct{}) (map[trinary.Hash]uint64, error) {

	// Calculate balances for targetIndex
	for milestoneIndex := solidMilestoneIndex; milestoneIndex > targetIndex; milestoneIndex-- {
		diff, err := tangle.GetLedgerDiffForMilestoneWithoutLocking(milestoneIndex, abortSignal)
		if err != nil {
			log.Panicf("CreateLocalSnapshot: %v", err)
		}

		for address, change := range diff {
			select {
			case <-abortSignal:
				return nil, ErrSnapshotCreationWasAborted
			default:
			}

			newBalance := int64(balances[address]) - change

			if newBalance < 0 {
				panic(fmt.Sprintf("CreateLocalSnapshot: Ledger diff for milestone %d creates negative balance for address %s: current %d, diff %d", milestoneIndex, address, balances[address], change))
			} else if newBalance == 0 {
				delete(balances, address)
			} else {
				balances[address] = uint64(newBalance)
			}
		}
	}
	return balances, nil
}

func checkSnapshotLimits(targetIndex milestone_index.MilestoneIndex, snapshotInfo *tangle.SnapshotInfo) error {

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	if targetIndex > (solidMilestoneIndex - SolidEntryPointCheckThresholdFuture) {
		return errors.Wrapf(ErrTargetIndexTooNew, "maximum: %d, actual: %d", solidMilestoneIndex-SolidEntryPointCheckThresholdFuture, targetIndex)
	}

	if targetIndex <= snapshotInfo.SnapshotIndex {
		return errors.Wrapf(ErrTargetIndexTooOld, "minimum: %d, actual: %d", snapshotInfo.SnapshotIndex, targetIndex)
	}

	if targetIndex-SolidEntryPointCheckThresholdPast < snapshotInfo.PruningIndex+1 {
		return errors.Wrapf(ErrTargetIndexTooOld, "minimum: %d, actual: %d", snapshotInfo.PruningIndex+1+SolidEntryPointCheckThresholdPast, targetIndex)
	}

	return nil
}

func createSnapshotFile(filePath string, lsh *localSnapshotHeader, abortSignal <-chan struct{}) ([]byte, error) {

	exportFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}
	defer exportFile.Close()

	// write into the file with an 8MB buffer
	fileBufWriter := bufio.NewWriterSize(exportFile, 4096*2)

	// write header, SEPs, seen milestones and ledger
	// with a WRONG spent addresses count
	if err := lsh.WriteToBuffer(fileBufWriter, abortSignal); err != nil {
		return nil, err
	}

	// flush remains of header and content without spent addresses to file
	if err := fileBufWriter.Flush(); err != nil {
		return nil, err
	}

	if tangle.GetSnapshotInfo().IsSpentAddressesEnabled() &&
		config.NodeConfig.GetBool(config.CfgSpentAddressesEnabled) {

		// stream spent addresses into the file
		spentAddressesCount, err := tangle.StreamSpentAddressesToWriter(fileBufWriter, abortSignal)
		if err != nil {
			return nil, err
		}

		if err := fileBufWriter.Flush(); err != nil {
			return nil, err
		}

		if spentAddressesCount > 0 {
			// seek to spent addresses count in the header:
			// 1 (version) + 49 (ms hash) + 4 (ms index) + 8 (ms timestamp) +
			// 4 (SEPs count) + 4 (seen ms count) + 4 (ledger entries) = 74
			if _, err := exportFile.Seek(74, 0); err != nil {
				return nil, err
			}

			// override 0 spent addresses count with actual count
			if err := binary.Write(exportFile, binary.LittleEndian, spentAddressesCount); err != nil {
				return nil, err
			}
		}
	}

	// seek back to the beginning of the file
	if _, err := exportFile.Seek(0, 0); err != nil {
		return nil, err
	}

	// compute sha256 of file
	lsHash := sha256.New()
	if _, err := io.Copy(lsHash, exportFile); err != nil {
		return nil, err
	}

	// write sha256 hash into the file
	sha256Hash := lsHash.Sum(nil)
	if err := binary.Write(exportFile, binary.LittleEndian, sha256Hash); err != nil {
		return nil, err
	}

	return sha256Hash, nil
}

func createLocalSnapshotWithoutLocking(targetIndex milestone_index.MilestoneIndex, filePath string, abortSignal <-chan struct{}) error {

	log.Infof("Creating local snapshot for targetIndex %d", targetIndex)

	ts := time.Now()

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	if err := checkSnapshotLimits(targetIndex, snapshotInfo); err != nil {
		return err
	}

	tangle.ReadLockSpentAddresses()
	defer tangle.ReadUnlockSpentAddresses()

	cachedTargetMs := tangle.GetMilestoneOrNil(targetIndex) // bundle +1
	if cachedTargetMs == nil {
		log.Panicf("CreateLocalSnapshot: Target milestone (%d) not found!", targetIndex)
	}
	defer cachedTargetMs.Release(true) // bundle -1

	tangle.ReadLockLedger()

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
	if !tangle.ContainsMilestone(solidMilestoneIndex) {
		log.Panicf("CreateLocalSnapshot: Solid milestone (%d) not found!", solidMilestoneIndex)
	}

	balances, ledgerMilestone, err := tangle.GetAllLedgerBalancesWithoutLocking(abortSignal)
	if err != nil {
		log.Panicf("CreateLocalSnapshot: GetAllLedgerBalances failed! %v", err)
	}

	if ledgerMilestone != solidMilestoneIndex {
		log.Panicf("CreateLocalSnapshot: LedgerMilestone wrong! %d/%d", ledgerMilestone, solidMilestoneIndex)
	}

	newBalances, err := getLedgerStateAtMilestone(balances, targetIndex, solidMilestoneIndex, abortSignal)
	tangle.ReadUnlockLedger()
	if err != nil {
		return err
	}

	newSolidEntryPoints, err := getSolidEntryPoints(targetIndex, abortSignal)
	if err != nil {
		return err
	}

	seenMilestones, err := getSeenMilestones(targetIndex, abortSignal)
	if err != nil {
		return err
	}

	cachedTargetMsTail := cachedTargetMs.GetBundle().GetTail() // tx +1
	defer cachedTargetMsTail.Release(true)                     // tx -1

	lsh := &localSnapshotHeader{
		msHash:           cachedTargetMs.GetBundle().GetTailHash(),
		msIndex:          targetIndex,
		msTimestamp:      cachedTargetMsTail.GetTransaction().GetTimestamp(),
		solidEntryPoints: newSolidEntryPoints,
		seenMilestones:   seenMilestones,
		balances:         newBalances,
	}

	filePathTmp := filePath + "_tmp"

	// Remove old temp file
	os.Remove(filePathTmp)

	hash, err := createSnapshotFile(filePathTmp, lsh, abortSignal)
	if err != nil {
		return err
	}

	if err := os.Rename(filePathTmp, filePath); err != nil {
		return err
	}

	tangle.WriteLockSolidEntryPoints()
	defer tangle.WriteUnlockSolidEntryPoints()

	tangle.ResetSolidEntryPoints()
	for solidEntryPoint, index := range newSolidEntryPoints {
		tangle.SolidEntryPointsAdd(solidEntryPoint, index)
	}
	tangle.StoreSolidEntryPoints()

	err = tangle.StoreSnapshotBalancesInDatabase(newBalances, targetIndex)
	if err != nil {
		return err
	}

	tangle.SetSnapshotInfo(&tangle.SnapshotInfo{
		Hash:          cachedTargetMs.GetBundle().GetMilestoneHash(),
		SnapshotIndex: targetIndex,
		PruningIndex:  snapshotInfo.PruningIndex,
		Timestamp:     cachedTargetMsTail.GetTransaction().GetTimestamp(),
		Metadata:      snapshotInfo.Metadata,
	})

	log.Infof("Created local snapshot for targetIndex %d (%x), took %v", targetIndex, hash, time.Since(ts))

	return nil
}

func CreateLocalSnapshot(targetIndex milestone_index.MilestoneIndex, filePath string, abortSignal <-chan struct{}) error {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()
	return createLocalSnapshotWithoutLocking(targetIndex, filePath, abortSignal)
}

type localSnapshotHeader struct {
	msHash              string
	msIndex             milestone_index.MilestoneIndex
	msTimestamp         int64
	solidEntryPoints    map[string]milestone_index.MilestoneIndex
	seenMilestones      map[string]milestone_index.MilestoneIndex
	balances            map[string]uint64
	spentAddressesCount int32
}

func (ls *localSnapshotHeader) WriteToBuffer(buf io.Writer, abortSignal <-chan struct{}) error {
	var err error

	if err = binary.Write(buf, binary.LittleEndian, SupportedLocalSnapshotFileVersions[0]); err != nil {
		return err
	}

	msHashBytes, err := trinary.TrytesToBytes(ls.msHash)
	if err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, msHashBytes[:49]); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, ls.msIndex); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, ls.msTimestamp); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, int32(len(ls.solidEntryPoints))); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, int32(len(ls.seenMilestones))); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, int32(len(ls.balances))); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, ls.spentAddressesCount); err != nil {
		return err
	}

	for hash, val := range ls.solidEntryPoints {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		addrBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, addrBytes[:49]); err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, val); err != nil {
			return err
		}
	}

	for hash, val := range ls.seenMilestones {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		addrBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, addrBytes[:49]); err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, val); err != nil {
			return err
		}
	}

	// ToDo: Don't convert to trinary at all
	for hash, val := range ls.balances {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		addrBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, addrBytes[:49]); err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, val); err != nil {
			return err
		}
	}

	return nil
}

func LoadSnapshotFromFile(filePath string) error {
	log.Info("Loading snapshot file...")

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	// check file version
	var fileVersion byte
	if err := binary.Read(file, binary.LittleEndian, &fileVersion); err != nil {
		return err
	}

	var supported bool
	for _, v := range SupportedLocalSnapshotFileVersions {
		if v == fileVersion {
			supported = true
			break
		}
	}
	if !supported {
		return errors.Wrapf(ErrUnsupportedLSFileVersion, "local snapshot file version is %d but this HORNET version only supports %v", fileVersion, SupportedLocalSnapshotFileVersions)
	}

	hashBuf := make([]byte, 49)
	if _, err := file.Read(hashBuf); err != nil {
		return err
	}

	tangle.WriteLockSolidEntryPoints()
	tangle.ResetSolidEntryPoints()

	// Genesis transaction
	tangle.SolidEntryPointsAdd(consts.NullHashTrytes, 0)

	var msIndex int32
	var msTimestamp int64
	var solidEntryPointsCount, seenMilestonesCount, ledgerEntriesCount, spentAddrsCount int32

	msHash, err := trinary.BytesToTrytes(hashBuf)
	if err != nil {
		return err
	}

	if err := binary.Read(file, binary.LittleEndian, &msIndex); err != nil {
		return err
	}

	if err := binary.Read(file, binary.LittleEndian, &msTimestamp); err != nil {
		return err
	}

	if err := binary.Read(file, binary.LittleEndian, &solidEntryPointsCount); err != nil {
		return err
	}

	if err := binary.Read(file, binary.LittleEndian, &seenMilestonesCount); err != nil {
		return err
	}

	if err := binary.Read(file, binary.LittleEndian, &ledgerEntriesCount); err != nil {
		return err
	}

	if err := binary.Read(file, binary.LittleEndian, &spentAddrsCount); err != nil {
		return err
	}

	tangle.SetSnapshotMilestone(msHash[:81], milestone_index.MilestoneIndex(msIndex), milestone_index.MilestoneIndex(msIndex), msTimestamp, spentAddrsCount != 0 && config.NodeConfig.GetBool("spentAddresses.enabled"))
	tangle.SolidEntryPointsAdd(msHash[:81], milestone_index.MilestoneIndex(msIndex))

	log.Info("Importing solid entry points")

	for i := 0; i < int(solidEntryPointsCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val int32

		if err := binary.Read(file, binary.LittleEndian, hashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
		}
		//ls.solidEntryPoints[hash[:81]] = val

		tangle.SolidEntryPointsAdd(hash[:81], milestone_index.MilestoneIndex(val))
	}

	tangle.StoreSolidEntryPoints()
	tangle.WriteUnlockSolidEntryPoints()

	log.Info("Importing seen milestones")

	for i := 0; i < int(seenMilestonesCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val int32

		if err := binary.Read(file, binary.LittleEndian, hashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
		}

		tangle.SetLatestSeenMilestoneIndexFromSnapshot(milestone_index.MilestoneIndex(val))
		gossip.Request([]trinary.Hash{hash[:81]}, milestone_index.MilestoneIndex(val))
	}

	log.Info("Importing ledger state")

	ledgerState := make(map[trinary.Hash]uint64)
	for i := 0; i < int(ledgerEntriesCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val uint64

		if err := binary.Read(file, binary.LittleEndian, hashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}
		ledgerState[hash[:81]] = val
	}

	err = tangle.StoreSnapshotBalancesInDatabase(ledgerState, milestone_index.MilestoneIndex(msIndex))
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "snapshot ledgerEntries: %s", err)
	}

	err = tangle.StoreLedgerBalancesInDatabase(ledgerState, milestone_index.MilestoneIndex(msIndex))
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
	}

	if config.NodeConfig.GetBool(config.CfgSpentAddressesEnabled) {
		log.Infof("Importing %d spent addresses. This can take a while...", spentAddrsCount)

		batchAmount := int(math.Ceil(float64(spentAddrsCount) / float64(SpentAddressesImportBatchSize)))
		for i := 0; i < batchAmount; i++ {
			if daemon.IsStopped() {
				return ErrSnapshotImportWasAborted
			}

			batchStart := int32(i * SpentAddressesImportBatchSize)
			batchEnd := batchStart + SpentAddressesImportBatchSize

			if batchEnd > spentAddrsCount {
				batchEnd = spentAddrsCount
			}

			for j := batchStart; j < batchEnd; j++ {

				spentAddrBuf := make([]byte, 49)
				err = binary.Read(file, binary.BigEndian, spentAddrBuf)
				if err != nil {
					return errors.Wrapf(ErrSnapshotImportFailed, "spentAddrs: %v", err)
				}

				tangle.MarkAddressAsSpentBinaryWithoutLocking(spentAddrBuf)
			}

			log.Infof("Processed %d/%d spent addresses", batchEnd, spentAddrsCount)
		}
	}

	log.Info("Finished loading snapshot")
	return nil
}
