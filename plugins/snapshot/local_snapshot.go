package snapshot

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

const (
	SolidEntryPointCheckThresholdPast   = 50
	SolidEntryPointCheckThresholdFuture = 50
)

var (
	SupportedLocalSnapshotFileVersions = []byte{5}

	ErrCritical                 = errors.New("critical error")
	ErrUnsupportedLSFileVersion = errors.New("unsupported local snapshot file version")
	ErrChildMsgNotFound         = errors.New("child message not found")
)

// isSolidEntryPoint checks whether any direct child of the given message was confirmed by a milestone which is above the target milestone.
func isSolidEntryPoint(messageID hornet.Hash, targetIndex milestone.Index) bool {

	for _, childMessageID := range tangle.GetChildrenMessageIDs(messageID) {
		cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(childMessageID) // meta +1
		if cachedMsgMeta == nil {
			// Ignore this message since it doesn't exist anymore
			log.Warnf(errors.Wrapf(ErrChildMsgNotFound, "msg ID: %v, child msg ID: %v", messageID.Hex(), childMessageID.Hex()).Error())
			continue
		}

		confirmed, at := cachedMsgMeta.GetMetadata().GetConfirmed()
		cachedMsgMeta.Release(true) // meta -1

		if confirmed && (at > targetIndex) {
			// confirmed by a later milestone than targetIndex => solidEntryPoint
			return true
		}
	}

	return false
}

// getMilestoneParents traverses a milestone and collects all messages that were confirmed by that milestone or newer
func getMilestoneParentMessageIDs(milestoneIndex milestone.Index, milestoneMessageID hornet.Hash, abortSignal <-chan struct{}) (hornet.Hashes, error) {

	var parentMessageIDs hornet.Hashes

	ts := time.Now()

	if err := dag.TraverseParents(milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *tangle.CachedMetadata) (bool, error) { // msg +1
			defer cachedMsgMeta.Release(true) // msg -1
			// collect all msg that were confirmed by that milestone or newer
			confirmed, at := cachedMsgMeta.GetMetadata().GetConfirmed()
			return (confirmed && at >= milestoneIndex), nil
		},
		// consumer
		func(cachedMsgMeta *tangle.CachedMetadata) error { // msg +1
			defer cachedMsgMeta.Release(true) // msg -1
			parentMessageIDs = append(parentMessageIDs, cachedMsgMeta.GetMetadata().GetMessageID())
			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		// the pruning target index is also a solid entry point => traverse it anyways
		true,
		abortSignal); err != nil {
		if err == tangle.ErrOperationAborted {
			return nil, ErrSnapshotCreationWasAborted
		}
	}

	log.Debugf("milestone walked (%d): parents: %v, collect: %v", milestoneIndex, len(parentMessageIDs), time.Since(ts))
	return parentMessageIDs, nil
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
	//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old msg as valid tips, which is not the case anymore.

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - SolidEntryPointCheckThresholdPast; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			return nil, ErrSnapshotCreationWasAborted
		default:
		}

		cachedMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			return nil, errors.Wrapf(ErrCritical, "milestone (%d) not found!", milestoneIndex)
		}

		// Get all parents of that milestone
		milestoneMessageID := cachedMilestone.GetMilestone().MessageID
		cachedMilestone.Release(true) // message -1

		parentMessageIDs, err := getMilestoneParentMessageIDs(milestoneIndex, milestoneMessageID, abortSignal)
		if err != nil {
			return nil, err
		}

		for _, parentMessageID := range parentMessageIDs {
			select {
			case <-abortSignal:
				return nil, ErrSnapshotCreationWasAborted
			default:
			}

			if isEntryPoint := isSolidEntryPoint(parentMessageID, targetIndex); isEntryPoint {
				cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(parentMessageID)
				if cachedMsgMeta == nil {
					return nil, errors.Wrapf(ErrCritical, "metadata (%v) not found!", parentMessageID.Hex())
				}

				confirmed, at := cachedMsgMeta.GetMetadata().GetConfirmed()
				if !confirmed {
					cachedMsgMeta.Release(true)
					return nil, errors.Wrapf(ErrCritical, "solid entry point (%v) not confirmed!", parentMessageID.Hex())
				}
				cachedMsgMeta.Release(true)

				solidEntryPoints[string(parentMessageID)] = at
			}
		}
	}

	return solidEntryPoints, nil
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
	if err := lsh.WriteToBuffer(fileBufWriter, abortSignal); err != nil {
		return nil, err
	}

	// flush remains of header and content to file
	if err := fileBufWriter.Flush(); err != nil {
		return nil, err
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

	cachedTargetMilestone := tangle.GetCachedMilestoneOrNil(targetIndex) // milestone +1
	if cachedTargetMilestone == nil {
		return errors.Wrapf(ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedTargetMilestone.Release(true) // milestone -1

	newBalances := make(map[string]uint64)
	/*
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
	*/

	newSolidEntryPoints, err := getSolidEntryPoints(targetIndex, abortSignal)
	if err != nil {
		return err
	}

	lsh := &localSnapshotHeader{
		milestoneMessageID: cachedTargetMilestone.GetMilestone().MessageID,
		msIndex:            targetIndex,
		msTimestamp:        cachedTargetMilestone.GetMilestone().Timestamp,
		solidEntryPoints:   newSolidEntryPoints,
		balances:           newBalances,
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
		/*
			// This has to be done before acquiring the SolidEntryPoints Lock, otherwise there is a race condition with "solidifyMilestone"
			// In "solidifyMilestone" the LedgerLock is acquired, but by traversing the tangle, the SolidEntryPoint Lock is also acquired.
			// ToDo: we should flush the caches here, just to be sure that all information before this local snapshot we stored in the persistence layer.
			err = tangle.StoreSnapshotBalancesInDatabase(newBalances, targetIndex)
			if err != nil {
				return errors.Wrap(ErrCritical, err.Error())
			}
		*/

		snapshotInfo.MilestoneMessageID = cachedTargetMilestone.GetMilestone().MessageID
		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = cachedTargetMilestone.GetMilestone().Timestamp
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
	milestoneMessageID hornet.Hash
	msIndex            milestone.Index
	msTimestamp        time.Time
	solidEntryPoints   map[string]milestone.Index
	balances           map[string]uint64
}

func (ls *localSnapshotHeader) WriteToBuffer(buf io.Writer, abortSignal <-chan struct{}) error {
	var err error

	if err = binary.Write(buf, binary.LittleEndian, SupportedLocalSnapshotFileVersions[0]); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, ls.milestoneMessageID[:49]); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, ls.msIndex); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, int64(ls.msTimestamp.Second())); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, int32(len(ls.solidEntryPoints))); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.LittleEndian, int32(len(ls.balances))); err != nil {
		return err
	}

	for messageID, val := range ls.solidEntryPoints {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		if err = binary.Write(buf, binary.LittleEndian, hornet.Hash(messageID)[:49]); err != nil {
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

	// ToDo:

	cooPublicKey, err := utils.ParseEd25519PublicKeyFromString(config.NodeConfig.GetString(config.CfgCoordinatorPublicKey))
	if err != nil {
		return err
	}

	tangle.SetSnapshotMilestone(cooPublicKey, hornet.NullMessageID, milestone.Index(1), milestone.Index(1), milestone.Index(1), time.Now())
	tangle.SetSolidMilestoneIndex(milestone.Index(1), false)

	return nil
	/*
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

		milestoneMessageID := make(hornet.Hash, 49)
		if _, err := file.Read(milestoneMessageID); err != nil {
			return err
		}

		tangle.WriteLockSolidEntryPoints()
		tangle.ResetSolidEntryPoints()

		var msIndex int32
		var msTimestamp int64
		var solidEntryPointsCount, seenMilestonesCount, ledgerEntriesCount int32

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

		cooPublicKey, err := utils.ParseEd25519PublicKeyFromString(config.NodeConfig.GetString(config.CfgCoordinatorPublicKey))
		if err != nil {
			return err
		}

		tangle.SetSnapshotMilestone(cooPublicKey, milestoneMessageID, milestone.Index(msIndex), milestone.Index(msIndex), milestone.Index(msIndex), time.Unix(msTimestamp, 0))
		tangle.SolidEntryPointsAdd(milestoneMessageID, milestone.Index(msIndex))

		log.Info("importing solid entry points")

		for i := 0; i < int(solidEntryPointsCount); i++ {
			if daemon.IsStopped() {
				return ErrSnapshotImportWasAborted
			}

			var val int32
			messageIDBuf := make(hornet.Hash, 49)

			if err := binary.Read(file, binary.LittleEndian, messageIDBuf); err != nil {
				return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
			}

			if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
				return errors.Wrapf(ErrSnapshotImportFailed, "solidEntryPoints: %v", err)
			}

			tangle.SolidEntryPointsAdd(messageIDBuf, milestone.Index(val))
		}

		tangle.StoreSolidEntryPoints()
		tangle.WriteUnlockSolidEntryPoints()

		log.Info("importing seen milestones")

		for i := 0; i < int(seenMilestonesCount); i++ {
			if daemon.IsStopped() {
				return ErrSnapshotImportWasAborted
			}

			var val int32
			messageIDBuf := make(hornet.Hash, 49)

			if err := binary.Read(file, binary.LittleEndian, messageIDBuf); err != nil {
				return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
			}

			if err := binary.Read(file, binary.LittleEndian, &val); err != nil {
				return errors.Wrapf(ErrSnapshotImportFailed, "seenMilestones: %v", err)
			}

			// request the milestone and prevent the request from being discarded from the request queue
			gossip.Request(messageIDBuf, milestone.Index(val), true)
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

		if total != iotago.TokenSupply {
			return errors.Wrapf(ErrInvalidBalance, "%d != %d", total, iotago.TokenSupply)
		}

		err = tangle.StoreSnapshotBalancesInDatabase(ledgerState, milestone.Index(msIndex))
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "snapshot ledgerEntries: %s", err)
		}

		err = tangle.StoreLedgerBalancesInDatabase(ledgerState, milestone.Index(msIndex))
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %v", err)
		}

		// set the solid milestone index based on the snapshot milestone
		tangle.SetSolidMilestoneIndex(milestone.Index(msIndex), false)

		log.Info("finished loading snapshot")

		tanglePlugin.Events.SnapshotMilestoneIndexChanged.Trigger(milestone.Index(msIndex))

		return nil
	*/

}
