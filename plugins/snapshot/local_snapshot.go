package snapshot

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	cuckoo "github.com/seiflotfy/cuckoofilter"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/daemon"

	"github.com/gohornet/hornet/packages/dag"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

const (
	SpentAddressesImportBatchSize            = 100000
	SolidEntryPointCheckThresholdPast        = 50
	SolidEntryPointCheckThresholdFuture      = 50
	SupportedLocalSnapshotFileVersion   byte = 3
)

var ErrUnsupportedLSFileVersion = errors.New("unsupported local snapshot file version")

// isSolidEntryPoint checks whether any direct approver of the given transaction was confirmed by a milestone which is above the target milestone.
func isSolidEntryPoint(txHash trinary.Hash, targetIndex milestone_index.MilestoneIndex) (bool, milestone_index.MilestoneIndex) {
	approvers, _ := tangle.GetApprovers(txHash)
	if approvers == nil {
		return false, 0
	}

	for _, approver := range approvers.GetHashes() {
		tx := tangle.GetCachedTransaction(approver) //+1
		if !tx.Exists() {
			tx.Release() //-1
			log.Panicf("isSolidEntryPoint: Transaction not found: %v", approver)
		}

		// HINT: Check for orphaned Tx as solid entry points is skipped in HORNET, since this operation is heavy and not necessary, and
		//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
		//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old tx as valid tips, which is not the case anymore.

		confirmed, at := tx.GetTransaction().GetConfirmed()
		tx.Release() //-1
		if confirmed && (at > targetIndex) {
			// confirmed by a later milestone than targetIndex => solidEntryPoint
			return true, at
		}
	}

	return false, 0
}

// getMilestoneApprovees traverses a milestone and collects all tx that were confirmed by that milestone or higher
func getMilestoneApprovees(milestoneIndex milestone_index.MilestoneIndex, milestoneTail *tangle.CachedTransaction, panicOnMissingTx bool, abortSignal <-chan struct{}) ([]trinary.Hash, error) {

	milestoneTail.RegisterConsumer() //+1
	defer milestoneTail.Release()    //-1

	ts := time.Now()

	txsToTraverse := make(map[string]struct{})
	txsChecked := make(map[string]struct{})
	var approvees []trinary.Hash
	txsToTraverse[milestoneTail.GetTransaction().GetHash()] = struct{}{}

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

			tx := tangle.GetCachedTransaction(txHash) //+1
			if !tx.Exists() {
				tx.Release() //-1
				if panicOnMissingTx {
					log.Panicf("getMilestoneApprovees: Transaction not found: %v", txHash)
				}

				// Go on if the tx is missing (needed for pruning of the database)
				continue
			}

			confirmed, at := tx.GetTransaction().GetConfirmed()
			if !confirmed {
				tx.Release() //-1
				log.Panicf("getMilestoneApprovees: Transaction must be confirmed: %v", txHash)
			}

			if at < milestoneIndex {
				// Ignore Tx that were confirmed by older milestones
				tx.Release() //-1
				continue
			}

			approvees = append(approvees, txHash)

			// Traverse the approvee
			txsToTraverse[tx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetTransaction().GetBranch()] = struct{}{}

			tx.Release() //-1
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

		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			log.Panicf("CreateLocalSnapshot: Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		msTail := ms.GetTail() //+1
		approvees, err := getMilestoneApprovees(milestoneIndex, msTail, true, abortSignal)
		msTail.Release() //-1
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
				tails, err := dag.FindAllTails(approvee)
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

		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			continue
		}
		seenMilestones[ms.GetTailHash()] = milestoneIndex
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

func createSnapshotFile(filePath string, lsh *localSnapshotHeader, abortSignal <-chan struct{}) error {

	var buf bytes.Buffer
	if err := lsh.WriteToBuffer(&buf, abortSignal); err != nil {
		return err
	}

	// write sha256 hash
	sha256Hash := sha256.Sum256(buf.Bytes())
	if err := binary.Write(&buf, binary.LittleEndian, sha256Hash); err != nil {
		return err
	}

	exportFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer exportFile.Close()

	gzipWriter := gzip.NewWriter(exportFile)
	defer gzipWriter.Close()

	if _, err = io.Copy(gzipWriter, &buf); err != nil {
		return err
	}
	return nil
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

	targetMilestone, _ := tangle.GetMilestone(targetIndex)
	if targetMilestone == nil {
		log.Panicf("CreateLocalSnapshot: Target milestone (%d) not found!", targetIndex)
	}

	tangle.ReadLockLedger()

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
	solidMilestone, _ := tangle.GetMilestone(solidMilestoneIndex)
	if solidMilestone == nil {
		log.Panicf("CreateLocalSnapshot: Solid milestone (%d) not found!", solidMilestoneIndex)
	}

	balances, ledgerMilestone, err := tangle.GetAllBalancesWithoutLocking(abortSignal)
	if err != nil {
		log.Panicf("CreateLocalSnapshot: GetAllBalances failed! %v", err)
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

	tangle.ReadLockSpentAddresses()
	defer tangle.ReadUnlockSpentAddresses()

	targetMilestoneTail := targetMilestone.GetTail() //+1

	lsh := &localSnapshotHeader{
		msHash:              targetMilestone.GetTailHash(),
		msIndex:             targetIndex,
		msTimestamp:         targetMilestoneTail.GetTransaction().GetTimestamp(),
		solidEntryPoints:    newSolidEntryPoints,
		seenMilestones:      seenMilestones,
		balances:            newBalances,
		spentAddressesCount: tangle.CountSpentAddressesEntries(),
		cuckooFilterBytes:   tangle.SerializedSpentAddressesCuckooFilter(),
	}

	filePathTmp := filePath + "_tmp"

	// Remove old temp file
	os.Remove(filePathTmp)

	if err := createSnapshotFile(filePathTmp, lsh, abortSignal); err != nil {
		targetMilestoneTail.Release() //-1
		return err
	}

	if err := os.Rename(filePathTmp, filePath); err != nil {
		targetMilestoneTail.Release() //-1
		return err
	}

	tangle.WriteLockSolidEntryPoints()
	defer tangle.WriteUnlockSolidEntryPoints()

	tangle.ResetSolidEntryPoints()
	for solidEntryPoint, index := range newSolidEntryPoints {
		tangle.SolidEntryPointsAdd(solidEntryPoint, index)
	}
	tangle.StoreSolidEntryPoints()

	tangle.SetSnapshotInfo(&tangle.SnapshotInfo{
		Hash:          targetMilestone.GetTailHash(),
		SnapshotIndex: targetIndex,
		PruningIndex:  snapshotInfo.PruningIndex,
		Timestamp:     targetMilestoneTail.GetTransaction().GetTimestamp(),
	})

	targetMilestoneTail.Release() //-1

	log.Infof("Creating local snapshot for targetIndex %d done, took %v", targetIndex, time.Since(ts))

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
	cuckooFilterBytes   []byte
}

func (ls *localSnapshotHeader) WriteToBuffer(buf io.Writer, abortSignal <-chan struct{}) error {
	var err error

	if err = binary.Write(buf, binary.LittleEndian, SupportedLocalSnapshotFileVersion); err != nil {
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

	if err := binary.Write(buf, binary.LittleEndian, int32(len(ls.cuckooFilterBytes))); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, ls.cuckooFilterBytes); err != nil {
		return err
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

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	// check file version
	var fileVersion byte
	if err := binary.Read(gzipReader, binary.LittleEndian, &fileVersion); err != nil {
		return err
	}

	if fileVersion != SupportedLocalSnapshotFileVersion {
		return errors.Wrapf(ErrUnsupportedLSFileVersion, "local snapshot file version is %d but this HORNET version only supports %d", fileVersion, SupportedLocalSnapshotFileVersion)
	}

	hashBuf := make([]byte, 49)
	if _, err := gzipReader.Read(hashBuf); err != nil {
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

	if err := binary.Read(gzipReader, binary.LittleEndian, &msIndex); err != nil {
		return err
	}

	if err := binary.Read(gzipReader, binary.LittleEndian, &msTimestamp); err != nil {
		return err
	}

	tangle.SetSnapshotMilestone(msHash[:81], milestone_index.MilestoneIndex(msIndex), milestone_index.MilestoneIndex(msIndex), msTimestamp)
	tangle.SolidEntryPointsAdd(msHash[:81], milestone_index.MilestoneIndex(msIndex))

	if err := binary.Read(gzipReader, binary.LittleEndian, &solidEntryPointsCount); err != nil {
		return err
	}

	if err := binary.Read(gzipReader, binary.LittleEndian, &seenMilestonesCount); err != nil {
		return err
	}

	if err := binary.Read(gzipReader, binary.LittleEndian, &ledgerEntriesCount); err != nil {
		return err
	}

	if err := binary.Read(gzipReader, binary.LittleEndian, &spentAddrsCount); err != nil {
		return err
	}

	log.Info("Importing solid entry points")

	for i := 0; i < int(solidEntryPointsCount); i++ {
		if daemon.IsStopped() {
			return ErrSnapshotImportWasAborted
		}

		var val int32

		if err := binary.Read(gzipReader, binary.LittleEndian, hashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
		}

		if err := binary.Read(gzipReader, binary.LittleEndian, &val); err != nil {
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

		if err := binary.Read(gzipReader, binary.LittleEndian, hashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
		}

		if err := binary.Read(gzipReader, binary.LittleEndian, &val); err != nil {
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

		if err := binary.Read(gzipReader, binary.LittleEndian, hashBuf); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		if err := binary.Read(gzipReader, binary.LittleEndian, &val); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}
		ledgerState[hash[:81]] = val
	}

	err = tangle.StoreBalancesInDatabase(ledgerState, milestone_index.MilestoneIndex(msIndex))
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
	}

	log.Infof("Deserializing spent addresses cuckoo filter containing %d addresses.", spentAddrsCount)

	// read serialized cuckoo filter byte size
	var cuckooFilterSize int32
	if err := binary.Read(gzipReader, binary.LittleEndian, &cuckooFilterSize); err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "couldn't deserialize cuckoo filter size: %v", err)
	}

	cuckooFilterData := make([]byte, cuckooFilterSize)
	if err := binary.Read(gzipReader, binary.LittleEndian, &cuckooFilterData); err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "couldn't read serialized cuckoo filter bytes: %v", err)
	}

	cuckooFilter, err := cuckoo.Decode(cuckooFilterData)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "couldn't reconstruct the cuckoo filter from the data within the snapshot file: %v", err)
	}

	if err := tangle.ImportSpentAddressesCuckooFilter(cuckooFilter); err != nil {
		return err
	}

	tangle.InitSpentAddressesCuckooFilter()

	log.Info("Finished loading snapshot")
	return nil
}
