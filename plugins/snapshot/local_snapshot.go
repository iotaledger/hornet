package snapshot

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/consts"

	"github.com/iotaledger/hive.go/daemon"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

const (
	SpentAddressesImportBatchSize       = 100000
	SolidEntryPointCheckThresholdPast   = 50
	SolidEntryPointCheckThresholdFuture = 50
)

var (
	SupportedLocalSnapshotFileVersions = []byte{4}

	ErrCritical                 = errors.New("critical error")
	ErrUnsupportedLSFileVersion = errors.New("unsupported local snapshot file version")
	ErrApproverTxNotFound       = errors.New("approver transaction not found")
)

// isSolidEntryPoint checks whether any direct approver of the given transaction was confirmed by a milestone which is above the target milestone.
func isSolidEntryPoint(txHash hornet.Hash, targetIndex milestone.Index) bool {

	for _, approverHash := range tangle.GetApproverHashes(txHash) {
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(approverHash) // meta +1
		if cachedTxMeta == nil {
			// Ignore this transaction since it doesn't exist anymore
			log.Warnf(errors.Wrapf(ErrApproverTxNotFound, "tx hash: %v, approver hash: %v", txHash.Trytes(), approverHash.Trytes()).Error())
			continue
		}

		// HINT: Check for orphaned Tx as solid entry points is skipped in HORNET, since this operation is heavy and not necessary, and
		//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
		//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old tx as valid tips, which is not the case anymore.

		confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed()
		cachedTxMeta.Release(true) // meta -1
		if confirmed && (at > targetIndex) {
			// confirmed by a later milestone than targetIndex => solidEntryPoint

			return true
		}
	}

	return false
}

// getMilestoneApprovees traverses a milestone and collects all tx that were confirmed by that milestone or higher
func getMilestoneApprovees(milestoneIndex milestone.Index, msTailTxHash hornet.Hash, abortSignal <-chan struct{}) (hornet.Hashes, error) {

	ts := time.Now()

	txsToTraverse := make(map[string]struct{})
	txsChecked := make(map[string]struct{})
	var approvees hornet.Hashes
	txsToTraverse[string(msTailTxHash)] = struct{}{}

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

			if tangle.SolidEntryPointsContain(hornet.Hash(txHash)) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(txHash)) // meta +1
			if cachedTxMeta == nil {
				return nil, errors.Wrapf(ErrCritical, "transaction not found: %v", hornet.Hash(txHash).Trytes())
			}

			if confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed(); confirmed {
				if at < milestoneIndex {
					// Ignore Tx that were confirmed by older milestones
					cachedTxMeta.Release(true) // meta -1
					continue
				}

				approvees = append(approvees, hornet.Hash(txHash))

				// Traverse the approvee
				txsToTraverse[string(cachedTxMeta.GetMetadata().GetTrunkHash())] = struct{}{}
				txsToTraverse[string(cachedTxMeta.GetMetadata().GetBranchHash())] = struct{}{}

				// Do not force release, since it is loaded again
				cachedTxMeta.Release() // meta -1

			} else {
				// Tx is not confirmed
				if cachedTxMeta.GetMetadata().IsTail() {
					cachedTxMeta.Release(true) // meta -1
					return nil, errors.Wrapf(ErrCritical, "transaction not confirmed: %v", hornet.Hash(txHash).Trytes())
				}

				// Search all referenced tails of this Tx (needed for correct SolidEntryPoint calculation).
				// This non-tail tx was not confirmed by the milestone, and could be referenced by the future cone.
				// Thats why we have to search all tail txs that get referenced by this incomplete bundle, to mark them as SEPs.
				tailTxs, err := dag.FindAllTails(hornet.Hash(txHash), false)
				if err != nil {
					cachedTxMeta.Release(true) // meta -1
					return nil, err
				}

				for tailTx := range tailTxs {
					txsToTraverse[tailTx] = struct{}{}
				}

				// Ignore this transaction in the cone because it is not confirmed
				cachedTxMeta.Release(true) // meta -1
			}
		}
	}

	log.Debugf("milestone walked (%d): approvees: %v, collect: %v", milestoneIndex, len(approvees), time.Since(ts))
	return approvees, nil
}

func shouldTakeSnapshot(solidMilestoneIndex milestone.Index) bool {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	var snapshotInterval milestone.Index
	if tangle.IsNodeSynced() {
		snapshotInterval = snapshotIntervalSynced
	} else {
		snapshotInterval = snapshotIntervalUnsynced
	}

	if (solidMilestoneIndex < snapshotDepth+snapshotInterval) || (solidMilestoneIndex-snapshotDepth) < snapshotInfo.PruningIndex+1+SolidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return solidMilestoneIndex-(snapshotDepth+snapshotInterval) >= snapshotInfo.SnapshotIndex
}

func getSolidEntryPoints(targetIndex milestone.Index, abortSignal <-chan struct{}) (map[string]milestone.Index, error) {

	solidEntryPoints := make(map[string]milestone.Index)

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
			return nil, errors.Wrapf(ErrCritical, "milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		msTailTxHash := cachedMs.GetBundle().GetTailHash()
		cachedMs.Release(true) // bundle -1

		approvees, err := getMilestoneApprovees(milestoneIndex, msTailTxHash, abortSignal)
		if err != nil {
			return nil, err
		}

		for _, approvee := range approvees {
			select {
			case <-abortSignal:
				return nil, ErrSnapshotCreationWasAborted
			default:
			}

			if isEntryPoint := isSolidEntryPoint(approvee, targetIndex); isEntryPoint {
				// A solid entry point should only be a tail transaction, otherwise the whole bundle can't be reproduced with a snapshot file
				tails, err := dag.FindAllTails(approvee, false)
				if err != nil {
					return nil, errors.Wrap(ErrCritical, err.Error())
				}

				for tailHash := range tails {
					cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(tailHash))
					if cachedTxMeta == nil {
						return nil, errors.Wrapf(ErrCritical, "metadata (%v) not found!", hornet.Hash(tailHash).Trytes())
					}

					confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed()
					if !confirmed {
						cachedTxMeta.Release(true)
						return nil, errors.Wrapf(ErrCritical, "solid entry point (%v) not confirmed!", hornet.Hash(tailHash).Trytes())
					}
					cachedTxMeta.Release(true)

					solidEntryPoints[tailHash] = at
				}
			}
		}
	}

	return solidEntryPoints, nil
}

func getSeenMilestones(targetIndex milestone.Index, abortSignal <-chan struct{}) (map[string]milestone.Index, error) {

	// Fill the list with seen milestones
	seenMilestones := make(map[string]milestone.Index)
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
		seenMilestones[string(cachedMs.GetBundle().GetTailHash())] = milestoneIndex
		cachedMs.Release(true) // bundle -1
	}
	return seenMilestones, nil
}

func checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *tangle.SnapshotInfo, checkSnapshotIndex bool) error {

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	if solidMilestoneIndex < SolidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum solid index: %d, actual solid index: %d", SolidEntryPointCheckThresholdFuture+1, solidMilestoneIndex)
	}

	minimumIndex := milestone.Index(SolidEntryPointCheckThresholdPast + 1)
	maximumIndex := milestone.Index(solidMilestoneIndex - SolidEntryPointCheckThresholdFuture)

	if checkSnapshotIndex && minimumIndex < snapshotInfo.SnapshotIndex+1 {
		minimumIndex = snapshotInfo.SnapshotIndex + 1
	}

	if minimumIndex < snapshotInfo.PruningIndex+1+SolidEntryPointCheckThresholdPast {
		minimumIndex = snapshotInfo.PruningIndex + 1 + SolidEntryPointCheckThresholdPast
	}

	if minimumIndex > maximumIndex {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum index (%d) exceeds maximum index (%d)", minimumIndex, maximumIndex)
	}

	if targetIndex > maximumIndex {
		return errors.Wrapf(ErrTargetIndexTooNew, "maximum: %d, actual: %d", maximumIndex, targetIndex)
	}

	if targetIndex < minimumIndex {
		return errors.Wrapf(ErrTargetIndexTooOld, "minimum: %d, actual: %d", minimumIndex, targetIndex)
	}

	return nil
}

func createSnapshotFile(filePath string, lsh *localSnapshotHeader, abortSignal <-chan struct{}) ([]byte, error) {

	if _, fileErr := os.Stat(filePath); os.IsNotExist(fileErr) {
		// create dir if it not exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
			return nil, err
		}
	}
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

func setIsSnapshotting(value bool) {
	statusLock.Lock()
	isSnapshotting = value
	statusLock.Unlock()
}

func createLocalSnapshotWithoutLocking(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {

	log.Infof("creating local snapshot for targetIndex %d", targetIndex)

	ts := time.Now()

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(ErrCritical, "no snapshot info found")
	}

	if err := checkSnapshotLimits(targetIndex, snapshotInfo, writeToDatabase); err != nil {
		return err
	}

	setIsSnapshotting(true)
	defer setIsSnapshotting(false)

	cachedTargetMs := tangle.GetMilestoneOrNil(targetIndex) // bundle +1
	if cachedTargetMs == nil {
		return errors.Wrapf(ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedTargetMs.Release(true) // bundle -1

	newBalances, ledgerIndex, err := tangle.GetLedgerStateForMilestone(targetIndex, abortSignal)
	if err != nil {
		if err == tangle.ErrOperationAborted {
			return err
		}
		return errors.Wrap(ErrCritical, err.Error())
	}

	if ledgerIndex != targetIndex {
		return errors.Wrapf(ErrCritical, "ledger index wrong! %d/%d", ledgerIndex, targetIndex)
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

	if writeToDatabase {
		// This has to be done before acquiring the SolidEntryPoints Lock, otherwise there is a race condition with "solidifyMilestone"
		// In "solidifyMilestone" the LedgerLock is acquired, but by traversing the tangle, the SolidEntryPoint Lock is also acquired.
		// ToDo: we should flush the caches here, just to be sure that all information before this local snapshot we stored in the persistence layer.
		err = tangle.StoreSnapshotBalancesInDatabase(newBalances, targetIndex)
		if err != nil {
			return errors.Wrap(ErrCritical, err.Error())
		}

		snapshotInfo.Hash = cachedTargetMs.GetBundle().GetMilestoneHash()
		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = cachedTargetMsTail.GetTransaction().GetTimestamp()
		tangle.SetSnapshotInfo(snapshotInfo)

		tanglePlugin.Events.SnapshotMilestoneIndexChanged.Trigger(targetIndex)
	}

	log.Infof("created local snapshot for target index %d (sha256: %x), took %v", targetIndex, hash, time.Since(ts))

	return nil
}

func CreateLocalSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()
	return createLocalSnapshotWithoutLocking(targetIndex, filePath, writeToDatabase, abortSignal)
}

type localSnapshotHeader struct {
	msHash              hornet.Hash
	msIndex             milestone.Index
	msTimestamp         int64
	solidEntryPoints    map[string]milestone.Index
	seenMilestones      map[string]milestone.Index
	balances            map[string]uint64
	spentAddressesCount int32
}

func (ls *localSnapshotHeader) WriteToBuffer(buf io.Writer, abortSignal <-chan struct{}) error {
	var err error

	if err = binary.Write(buf, binary.LittleEndian, SupportedLocalSnapshotFileVersions[0]); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, ls.msHash[:49]); err != nil {
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

		if err = binary.Write(buf, binary.LittleEndian, hornet.Hash(hash)[:49]); err != nil {
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

		if err = binary.Write(buf, binary.LittleEndian, hornet.Hash(hash)[:49]); err != nil {
			return err
		}

		if err = binary.Write(buf, binary.LittleEndian, val); err != nil {
			return err
		}
	}

	for addr, val := range ls.balances {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		if err = binary.Write(buf, binary.LittleEndian, hornet.Hash(addr)[:49]); err != nil {
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

	msHash := make(hornet.Hash, 49)
	if _, err := file.Read(msHash); err != nil {
		return err
	}

	tangle.WriteLockSolidEntryPoints()
	tangle.ResetSolidEntryPoints()

	var msIndex int32
	var msTimestamp int64
	var solidEntryPointsCount, seenMilestonesCount, ledgerEntriesCount, spentAddrsCount int32

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

	coordinatorAddress := hornet.HashFromAddressTrytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress))
	tangle.SetSnapshotMilestone(coordinatorAddress, msHash, milestone.Index(msIndex), milestone.Index(msIndex), milestone.Index(msIndex), msTimestamp, spentAddrsCount != 0 && config.NodeConfig.GetBool("spentAddresses.enabled"))
	tangle.SolidEntryPointsAdd(msHash, milestone.Index(msIndex))
	tangle.SetLatestSeenMilestoneIndexFromSnapshot(milestone.Index(msIndex))

	log.Info("importing solid entry points")

	for i := 0; i < int(solidEntryPointsCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val int32
		txHashBuf := make(hornet.Hash, 49)

		if err := binary.Read(file, binary.LittleEndian, txHashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
		}

		tangle.SolidEntryPointsAdd(txHashBuf, milestone.Index(val))
	}

	tangle.StoreSolidEntryPoints()
	tangle.WriteUnlockSolidEntryPoints()

	log.Info("importing seen milestones")

	for i := 0; i < int(seenMilestonesCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val int32
		txHashBuf := make(hornet.Hash, 49)

		if err := binary.Read(file, binary.LittleEndian, txHashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
		}

		tangle.SetLatestSeenMilestoneIndexFromSnapshot(milestone.Index(val))
		// request the milestone and prevent the request from being discarded from the request queue
		gossip.Request(txHashBuf, milestone.Index(val), true)
	}

	log.Info("importing ledger state")

	ledgerState := make(map[string]uint64)
	for i := 0; i < int(ledgerEntriesCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val uint64
		addrBuf := make(hornet.Hash, 49)

		if err := binary.Read(file, binary.LittleEndian, addrBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		ledgerState[string(addrBuf)] = val
	}

	var total uint64
	for _, value := range ledgerState {
		total += value
	}

	if total != consts.TotalSupply {
		return errors.Wrapf(ErrInvalidBalance, "%d != %d", total, consts.TotalSupply)
	}

	err = tangle.StoreSnapshotBalancesInDatabase(ledgerState, milestone.Index(msIndex))
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "snapshot ledgerEntries: %s", err)
	}

	err = tangle.StoreLedgerBalancesInDatabase(ledgerState, milestone.Index(msIndex))
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
	}

	if config.NodeConfig.GetBool(config.CfgSpentAddressesEnabled) {
		log.Infof("importing %d spent addresses. this can take a while...", spentAddrsCount)

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

				spentAddrBuf := make(hornet.Hash, 49)
				err = binary.Read(file, binary.BigEndian, spentAddrBuf)
				if err != nil {
					return errors.Wrapf(ErrSnapshotImportFailed, "spentAddrs: %v", err)
				}

				tangle.MarkAddressAsSpentWithoutLocking(spentAddrBuf)
			}

			log.Infof("processed %d/%d spent addresses", batchEnd, spentAddrsCount)
		}
	}

	// set the solid milestone index based on the snapshot milestone
	tangle.SetSolidMilestoneIndex(milestone.Index(msIndex), false)

	log.Info("finished loading snapshot")

	tanglePlugin.Events.SnapshotMilestoneIndexChanged.Trigger(milestone.Index(msIndex))

	return nil
}
