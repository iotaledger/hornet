package snapshot

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/tangle"
)

var (
	// Returned when a critical error stops the execution of a task.
	ErrCritical = errors.New("critical error")
	// Returned when unsupported snapshot data is read.
	ErrUnsupportedSnapshot = errors.New("unsupported snapshot data")
	// Returned when a child message wasn't found.
	ErrChildMsgNotFound = errors.New("child message not found")
	// Returned when the milestone diff that should be applied is not the current or next milestone.
	ErrWrongMilestoneDiffIndex = errors.New("wrong milestone diff index")
	// Returned when the final milestone after loading the snapshot is not equal to the solid entry point index.
	ErrFinalLedgerIndexDoesNotMatchSEPIndex = errors.New("final ledger index does not match solid entry point index")

	ErrNoSnapshotSpecified                   = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL                 = errors.New("no download URL specified for snapshot files in config")
	ErrSnapshotDownloadWasAborted            = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource         = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotCreationWasAborted            = errors.New("operation was aborted")
	ErrSnapshotCreationFailed                = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew                     = errors.New("snapshot target is too new")
	ErrTargetIndexTooOld                     = errors.New("snapshot target is too old")
	ErrNotEnoughHistory                      = errors.New("not enough history")
	ErrNoPruningNeeded                       = errors.New("no pruning needed")
	ErrPruningAborted                        = errors.New("pruning was aborted")
	ErrExistingDeltaSnapshotWrongLedgerIndex = errors.New("existing delta ledger snapshot has wrong ledger index")
)

type solidEntryPoint struct {
	messageID *hornet.MessageID
	index     milestone.Index
}

// Snapshot handles reading and writing snapshot data.
type Snapshot struct {
	shutdownCtx                         context.Context
	log                                 *logger.Logger
	storage                             *storage.Storage
	tangle                              *tangle.Tangle
	utxo                                *utxo.Manager
	snapshotFullPath                    string
	snapshotDeltaPath                   string
	solidEntryPointCheckThresholdPast   milestone.Index
	solidEntryPointCheckThresholdFuture milestone.Index
	additionalPruningThreshold          milestone.Index
	snapshotDepth                       milestone.Index
	snapshotIntervalSynced              milestone.Index
	snapshotIntervalUnsynced            milestone.Index
	pruningEnabled                      bool
	pruningDelay                        milestone.Index

	snapshotLock   syncutils.Mutex
	statusLock     syncutils.RWMutex
	isSnapshotting bool
	isPruning      bool
}

// New creates a new snapshot instance.
func New(shutdownCtx context.Context,
	log *logger.Logger,
	storage *storage.Storage,
	tangle *tangle.Tangle,
	utxo *utxo.Manager,
	snapshotFullPath string,
	snapshotDeltaPath string,
	solidEntryPointCheckThresholdPast milestone.Index,
	solidEntryPointCheckThresholdFuture milestone.Index,
	additionalPruningThreshold milestone.Index,
	snapshotDepth milestone.Index,
	snapshotIntervalSynced milestone.Index,
	snapshotIntervalUnsynced milestone.Index,
	pruningEnabled bool,
	pruningDelay milestone.Index) *Snapshot {

	return &Snapshot{
		shutdownCtx:                         shutdownCtx,
		log:                                 log,
		storage:                             storage,
		tangle:                              tangle,
		utxo:                                utxo,
		snapshotFullPath:                    snapshotFullPath,
		snapshotDeltaPath:                   snapshotDeltaPath,
		solidEntryPointCheckThresholdPast:   solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture: solidEntryPointCheckThresholdFuture,
		additionalPruningThreshold:          additionalPruningThreshold,
		snapshotDepth:                       snapshotDepth,
		snapshotIntervalSynced:              snapshotIntervalSynced,
		snapshotIntervalUnsynced:            snapshotIntervalUnsynced,
		pruningEnabled:                      pruningEnabled,
		pruningDelay:                        pruningDelay,
	}
}

func (s *Snapshot) IsSnapshottingOrPruning() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.isSnapshotting || s.isPruning
}

// isSolidEntryPoint checks whether any direct child of the given message was referenced by a milestone which is above the target milestone.
func (s *Snapshot) isSolidEntryPoint(messageID *hornet.MessageID, targetIndex milestone.Index) bool {

	for _, childMessageID := range s.storage.GetChildrenMessageIDs(messageID) {
		cachedMsgMeta := s.storage.GetCachedMessageMetadataOrNil(childMessageID) // meta +1
		if cachedMsgMeta == nil {
			// Ignore this message since it doesn't exist anymore
			s.log.Warnf("%s, msg ID: %v, child msg ID: %v", ErrChildMsgNotFound, messageID.Hex(), childMessageID.Hex())
			continue
		}

		referenced, at := cachedMsgMeta.GetMetadata().GetReferenced()
		cachedMsgMeta.Release(true) // meta -1

		if referenced && (at > targetIndex) {
			// referenced by a later milestone than targetIndex => solidEntryPoint
			return true
		}
	}

	return false
}

// getMilestoneParents traverses a milestone and collects all messages that were referenced by that milestone or newer.
func (s *Snapshot) getMilestoneParentMessageIDs(milestoneIndex milestone.Index, milestoneMessageID *hornet.MessageID, abortSignal <-chan struct{}) (hornet.MessageIDs, error) {

	var parentMessageIDs hornet.MessageIDs

	ts := time.Now()

	if err := dag.TraverseParents(s.storage, milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // msg +1
			defer cachedMsgMeta.Release(true) // msg -1
			// collect all msg that were referenced by that milestone or newer
			referenced, at := cachedMsgMeta.GetMetadata().GetReferenced()
			return referenced && at >= milestoneIndex, nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error { // msg +1
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
		if err == common.ErrOperationAborted {
			return nil, ErrSnapshotCreationWasAborted
		}
	}

	s.log.Debugf("milestone walked (%d): parents: %v, collect: %v", milestoneIndex, len(parentMessageIDs), time.Since(ts))
	return parentMessageIDs, nil
}

func (s *Snapshot) shouldTakeSnapshot(solidMilestoneIndex milestone.Index) bool {

	snapshotInfo := s.storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		s.log.Panic("No snapshotInfo found!")
	}

	var snapshotInterval milestone.Index
	if s.storage.IsNodeSynced() {
		snapshotInterval = s.snapshotIntervalSynced
	} else {
		snapshotInterval = s.snapshotIntervalUnsynced
	}

	if (solidMilestoneIndex < s.snapshotDepth+snapshotInterval) || (solidMilestoneIndex-s.snapshotDepth) < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return solidMilestoneIndex-(s.snapshotDepth+snapshotInterval) >= snapshotInfo.SnapshotIndex
}

func (s *Snapshot) forEachSolidEntryPoint(targetIndex milestone.Index, abortSignal <-chan struct{}, solidEntryPointConsumer func(sep *solidEntryPoint) bool) error {

	solidEntryPoints := make(map[string]milestone.Index)

	// HINT: Check if "old solid entry points are still valid" is skipped in HORNET,
	//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
	//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old msg as valid tips, which is not the case anymore.

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - s.solidEntryPointCheckThresholdPast; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		cachedMilestone := s.storage.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			return errors.Wrapf(ErrCritical, "milestone (%d) not found!", milestoneIndex)
		}

		// Get all parents of that milestone
		milestoneMessageID := cachedMilestone.GetMilestone().MessageID
		cachedMilestone.Release(true) // message -1

		parentMessageIDs, err := s.getMilestoneParentMessageIDs(milestoneIndex, milestoneMessageID, abortSignal)
		if err != nil {
			return err
		}

		for _, parentMessageID := range parentMessageIDs {
			select {
			case <-abortSignal:
				return ErrSnapshotCreationWasAborted
			default:
			}

			if isEntryPoint := s.isSolidEntryPoint(parentMessageID, targetIndex); !isEntryPoint {
				continue
			}

			cachedMsgMeta := s.storage.GetCachedMessageMetadataOrNil(parentMessageID)
			if cachedMsgMeta == nil {
				return errors.Wrapf(ErrCritical, "metadata (%v) not found!", parentMessageID.Hex())
			}

			referenced, at := cachedMsgMeta.GetMetadata().GetReferenced()
			if !referenced {
				cachedMsgMeta.Release(true)
				return errors.Wrapf(ErrCritical, "solid entry point (%v) not referenced!", parentMessageID.Hex())
			}
			cachedMsgMeta.Release(true)

			parentMessageIDMapKey := parentMessageID.MapKey()
			if _, exists := solidEntryPoints[parentMessageIDMapKey]; !exists {
				solidEntryPoints[parentMessageIDMapKey] = at
				if !solidEntryPointConsumer(&solidEntryPoint{messageID: parentMessageID, index: at}) {
					return ErrSnapshotCreationWasAborted
				}
			}
		}
	}

	return nil
}

func (s *Snapshot) checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *storage.SnapshotInfo, checkSnapshotIndex bool) error {

	solidMilestoneIndex := s.storage.GetSolidMilestoneIndex()

	if solidMilestoneIndex < s.solidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum solid index: %d, actual solid index: %d", s.solidEntryPointCheckThresholdFuture+1, solidMilestoneIndex)
	}

	minimumIndex := s.solidEntryPointCheckThresholdPast + 1
	maximumIndex := solidMilestoneIndex - s.solidEntryPointCheckThresholdFuture

	if checkSnapshotIndex && minimumIndex < snapshotInfo.SnapshotIndex+1 {
		minimumIndex = snapshotInfo.SnapshotIndex + 1
	}

	if minimumIndex < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		minimumIndex = snapshotInfo.PruningIndex + 1 + s.solidEntryPointCheckThresholdPast
	}

	switch {
	case minimumIndex > maximumIndex:
		return errors.Wrapf(ErrNotEnoughHistory, "minimum index (%d) exceeds maximum index (%d)", minimumIndex, maximumIndex)
	case targetIndex > maximumIndex:
		return errors.Wrapf(ErrTargetIndexTooNew, "maximum: %d, actual: %d", maximumIndex, targetIndex)
	case targetIndex < minimumIndex:
		return errors.Wrapf(ErrTargetIndexTooOld, "minimum: %d, actual: %d", minimumIndex, targetIndex)
	}

	return nil
}

func (s *Snapshot) setIsSnapshotting(value bool) {
	s.statusLock.Lock()
	s.isSnapshotting = value
	s.statusLock.Unlock()
}

// CreateFullSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) CreateFullSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createSnapshotWithoutLocking(Full, targetIndex, filePath, writeToDatabase, abortSignal)
}

// CreateDeltaSnapshot creates a delta snapshot for the given target milestone index.
func (s *Snapshot) CreateDeltaSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createSnapshotWithoutLocking(Delta, targetIndex, filePath, writeToDatabase, abortSignal)
}

// returns a producer which produces solid entry points.
func newSEPsProducer(s *Snapshot, targetIndex milestone.Index, abortSignal <-chan struct{}) SEPProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		// calculate solid entry points for the target index
		if err := s.forEachSolidEntryPoint(targetIndex, abortSignal, func(sep *solidEntryPoint) bool {
			prodChan <- sep.messageID
			return true
		}); err != nil {
			errChan <- err
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)
	return func() (*hornet.MessageID, error) {
		obj, err := binder()
		if obj == nil || err != nil {
			return nil, err
		}
		return obj.(*hornet.MessageID), nil
	}
}

// returns a producer which produces unspent outputs which exist for the current solid milestone.
func newLSMIUTXOProducer(utxoManager *utxo.Manager) OutputProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		if err := utxoManager.ForEachUnspentOutputWithoutLocking(func(output *utxo.Output) bool {
			prodChan <- &Output{MessageID: *output.MessageID(), OutputID: *output.OutputID(), Address: output.Address(), Amount: output.Amount()}
			return true
		}); err != nil {
			errChan <- err
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)
	return func() (*Output, error) {
		obj, err := binder()
		if obj == nil || err != nil {
			return nil, err
		}
		return obj.(*Output), nil
	}
}

// MsDiffDirection determines the milestone diff direction.
type MsDiffDirection byte

const (
	// MsDiffDirectionBackwards defines to produce milestone diffs in backwards direction.
	MsDiffDirectionBackwards MsDiffDirection = iota
	// MsDiffDirectionOnwards defines to produce milestone diffs in onwards direction.
	MsDiffDirectionOnwards
)

// returns an iterator producing milestone indices with the given direction from/to the milestone range.
func newMsIndexIterator(direction MsDiffDirection, ledgerIndex milestone.Index, targetIndex milestone.Index) func() (msIndex milestone.Index, done bool) {
	var firstPassDone bool
	switch direction {
	case MsDiffDirectionOnwards:
		// we skip the diff of the ledger milestone
		msIndex := ledgerIndex + 1
		return func() (milestone.Index, bool) {
			if firstPassDone {
				msIndex++
			}
			if msIndex > targetIndex {
				return 0, true
			}
			firstPassDone = true
			return msIndex, false
		}

	case MsDiffDirectionBackwards:
		// targetIndex is not included, since we only need the diff of targetIndex+1 to
		// calculate the ledger index of targetIndex
		msIndex := ledgerIndex
		return func() (milestone.Index, bool) {
			if firstPassDone {
				msIndex--
			}
			if msIndex == targetIndex {
				return 0, true
			}
			firstPassDone = true
			return msIndex, false
		}

	default:
		panic("invalid milestone diff direction")
	}
}

// returns a milestone diff producer which first reads out milestone diffs from an existing delta
// snapshot file and then the remaining diffs from the database up to the target index.
func newMsDiffsProducerDeltaFileAndDatabase(snapshotDeltaPath string, utxoManager *utxo.Manager, ledgerIndex milestone.Index, targetIndex milestone.Index) (MilestoneDiffProducerFunc, error) {
	prevDeltaFileMsDiffsProducer, err := newMsDiffsFromPreviousDeltaSnapshot(snapshotDeltaPath, ledgerIndex)
	if err != nil {
		return nil, err
	}

	var prevDeltaMsDiffProducerFinished bool
	var prevDeltaUpToIndex milestone.Index
	var dbMsDiffProducer MilestoneDiffProducerFunc
	return func() (*MilestoneDiff, error) {
		if prevDeltaMsDiffProducerFinished {
			return dbMsDiffProducer()
		}

		// consume existing delta snapshot data
		msDiff, err := prevDeltaFileMsDiffsProducer()
		if err != nil {
			return nil, err
		}

		if msDiff != nil {
			prevDeltaUpToIndex = msDiff.MilestoneIndex
			return msDiff, nil
		}

		// TODO: check whether previous snapshot already hit the target index?

		prevDeltaMsDiffProducerFinished = true
		dbMsDiffProducer = newMsDiffsProducer(utxoManager, MsDiffDirectionOnwards, prevDeltaUpToIndex, targetIndex)
		return dbMsDiffProducer()
	}, nil
}

// returns a milestone diff producer which reads out the milestone diffs from an existing delta snapshot file.
// the existing delta snapshot file is closed as soon as its milestone diffs are read.
func newMsDiffsFromPreviousDeltaSnapshot(snapshotDeltaPath string, originLedgerIndex milestone.Index) (MilestoneDiffProducerFunc, error) {
	existingDeltaFile, err := os.OpenFile(snapshotDeltaPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("unable to read previous delta snapshot file for milestone diffs: %w", err)
	}

	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		defer existingDeltaFile.Close()

		if err := StreamSnapshotDataFrom(existingDeltaFile,
			func(header *ReadFileHeader) error {
				// check that the ledger index matches
				if header.LedgerMilestoneIndex != originLedgerIndex {
					return fmt.Errorf("%w: wanted %d but got %d", ErrExistingDeltaSnapshotWrongLedgerIndex, originLedgerIndex, header.LedgerMilestoneIndex)
				}
				return nil
			},
			func(id *hornet.MessageID) error {
				// we don't care about solid entry points
				return nil
			}, nil,
			func(milestoneDiff *MilestoneDiff) error {
				prodChan <- milestoneDiff
				return nil
			},
		); err != nil {
			errChan <- err
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)
	return func() (*MilestoneDiff, error) {
		obj, err := binder()
		if obj == nil || err != nil {
			return nil, err
		}
		return obj.(*MilestoneDiff), nil
	}, nil
}

// returns a producer which produces milestone diffs from/to with the given direction.
func newMsDiffsProducer(utxoManager *utxo.Manager, direction MsDiffDirection, ledgerMilestoneIndex milestone.Index, targetIndex milestone.Index) MilestoneDiffProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		msIndexIterator := newMsIndexIterator(direction, ledgerMilestoneIndex, targetIndex)

		var done bool
		var msIndex milestone.Index

		for msIndex, done = msIndexIterator(); !done; msIndex, done = msIndexIterator() {
			newOutputs, newSpents, err := utxoManager.GetMilestoneDiffsWithoutLocking(msIndex)
			if err != nil {
				errChan <- err
				close(prodChan)
				close(errChan)
				return
			}

			var createdOutputs []*Output
			var consumedOutputs []*Spent

			for _, output := range newOutputs {
				createdOutputs = append(createdOutputs, &Output{
					MessageID: *output.MessageID(), OutputID: *output.OutputID(),
					Address: output.Address(), Amount: output.Amount()})
			}

			for _, spent := range newSpents {
				consumedOutputs = append(consumedOutputs, &Spent{Output: Output{
					MessageID: *spent.MessageID(), OutputID: *spent.OutputID(),
					Address: spent.Address(), Amount: spent.Amount()},
					TargetTransactionID: *spent.TargetTransactionID()},
				)
			}

			prodChan <- &MilestoneDiff{
				MilestoneIndex: msIndex, Created: createdOutputs,
				Consumed: consumedOutputs,
			}
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)
	return func() (*MilestoneDiff, error) {
		obj, err := binder()
		if obj == nil || err != nil {
			return nil, err
		}
		return obj.(*MilestoneDiff), nil
	}
}

// returns a function which tries to read from the given producer and error channels up on each invocation.
func producerFromChannels(prodChan <-chan interface{}, errChan <-chan error) func() (interface{}, error) {
	return func() (interface{}, error) {
		select {
		case err, ok := <-errChan:
			if !ok {
				return nil, nil
			}
			return nil, err
		case obj, ok := <-prodChan:
			if !ok {
				return nil, nil
			}
			return obj, nil
		}
	}
}

// reads out the index of the milestone which currently represents the ledger state.
func (s *Snapshot) readLSMLedgerIndex() (milestone.Index, error) {
	ledgerMilestoneIndex, err := s.utxo.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return 0, fmt.Errorf("unable to read current ledger index: %w", err)
	}

	cachedMilestone := s.storage.GetCachedMilestoneOrNil(ledgerMilestoneIndex)
	if cachedMilestone == nil {
		return 0, errors.Wrapf(ErrCritical, "milestone (%d) not found!", ledgerMilestoneIndex)
	}
	cachedMilestone.Release(true)
	return ledgerMilestoneIndex, nil
}

// reads out the snapshot milestone index from the full snapshot file.
func (s *Snapshot) readSnapshotIndexFromFullSnapshotFile() (milestone.Index, error) {
	fullSnapshotHeader, err := ReadSnapshotHeader(s.snapshotFullPath)
	if err != nil {
		return 0, fmt.Errorf("unable to read full snapshot header for origin snapshot milestone index: %w", err)
	}

	// note that a full snapshot contains the ledger to the lsmi of the node which generated it,
	// however, the state is rolled backed to the snapshot index, therefore, the snapshot index
	// is the actual point from which on the delta snapshot should contain milestone diffs
	return fullSnapshotHeader.SEPMilestoneIndex, nil
}

// creates the temp file into which to write the snapshot data into.
func (s *Snapshot) createTempFile(filePath string) (*os.File, string, error) {
	filePathTmp := filePath + "_tmp"
	_ = os.Remove(filePathTmp)

	lsFile, err := os.OpenFile(filePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create tmp snapshot file: %w", err)
	}
	return lsFile, filePathTmp, nil
}

// renames the given temp file to the final file name.
func (s *Snapshot) renameTempFile(tempFile *os.File, tempFilePath string, filePath string) error {
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("unable to close snapshot file: %w", err)
	}
	if err := os.Rename(tempFilePath, filePath); err != nil {
		return fmt.Errorf("unable to rename temp snapshot file: %w", err)
	}
	return nil
}

// returns the timestamp of the target milestone.
func (s *Snapshot) readTargetMilestoneTimestamp(targetIndex milestone.Index) (time.Time, error) {
	cachedTargetMilestone := s.storage.GetCachedMilestoneOrNil(targetIndex) // milestone +1
	if cachedTargetMilestone == nil {
		return time.Time{}, errors.Wrapf(ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	ts := cachedTargetMilestone.GetMilestone().Timestamp
	cachedTargetMilestone.Release(true) // milestone -1
	return ts, nil
}

// creates a snapshot file by streaming data from the database into a snapshot file.
func (s *Snapshot) createSnapshotWithoutLocking(snapshotType Type, targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	s.log.Infof("creating %s snapshot for targetIndex %d", snapshotNames[snapshotType], targetIndex)
	ts := time.Now()

	s.setIsSnapshotting(true)
	defer s.setIsSnapshotting(false)

	s.utxo.ReadLockLedger()
	defer s.utxo.ReadUnlockLedger()

	snapshotInfo := s.storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(ErrCritical, "no snapshot info found")
	}

	targetMsTimestamp, err := s.readTargetMilestoneTimestamp(targetIndex)
	if err != nil {
		return err
	}

	if err := s.checkSnapshotLimits(targetIndex, snapshotInfo, writeToDatabase); err != nil {
		return err
	}

	header := &FileHeader{
		Version:           SupportedFormatVersion,
		Type:              snapshotType,
		NetworkID:         snapshotInfo.NetworkID,
		SEPMilestoneIndex: targetIndex,
	}

	snapshotFile, tempFilePath, err := s.createTempFile(filePath)
	if err != nil {
		return err
	}

	// generate producers
	var utxoProducer OutputProducerFunc
	var milestoneDiffProducer MilestoneDiffProducerFunc
	switch snapshotType {
	case Full:
		// ledger index corresponds to the latest lsmi
		header.LedgerMilestoneIndex, err = s.readLSMLedgerIndex()
		if err != nil {
			return err
		}

		// a full snapshot contains the ledger UTXOs as of the LSMI
		// and the milestone diffs from the LSMI back to the target index (excluding the target index)
		utxoProducer = newLSMIUTXOProducer(s.utxo)
		milestoneDiffProducer = newMsDiffsProducer(s.utxo, MsDiffDirectionBackwards, header.LedgerMilestoneIndex, targetIndex)

	case Delta:
		// ledger index corresponds to the origin snapshot snapshot ledger.
		// this will return an error if the full snapshot file is not available
		header.LedgerMilestoneIndex, err = s.readSnapshotIndexFromFullSnapshotFile()
		if err != nil {
			return err
		}

		// a delta snapshot contains the milestone diffs from a full snapshot's snapshot index onwards
		switch {
		case snapshotInfo.SnapshotIndex == snapshotInfo.PruningIndex:
			// when booting up the first time on a full snapshot or in combination with a delta
			// snapshot, this indices will be the same.
			// while booting up with a delta snapshot would mean that the data is available in that file for the
			// next snapshot, we still only take the ms diffs from the database for at most 1 delta snapshot.
			fallthrough
		case snapshotInfo.PruningIndex < header.LedgerMilestoneIndex:
			// we have the needed milestone diffs in the database
			milestoneDiffProducer = newMsDiffsProducer(s.utxo, MsDiffDirectionOnwards, header.LedgerMilestoneIndex, targetIndex)
		default:
			// as the needed milestone diffs are pruned from the database, we need to use
			// the previous delta snapshot file to extract those in conjunction with what the database has available
			milestoneDiffProducer, err = newMsDiffsProducerDeltaFileAndDatabase(s.snapshotDeltaPath, s.utxo, header.LedgerMilestoneIndex, targetIndex)
			if err != nil {
				return err
			}
		}
	}

	// stream data into snapshot file
	if err := StreamSnapshotDataTo(snapshotFile, uint64(ts.Unix()), header, newSEPsProducer(s, targetIndex, abortSignal), utxoProducer, milestoneDiffProducer); err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[snapshotType], err)
	}

	// finalize file
	if err := s.renameTempFile(snapshotFile, tempFilePath, filePath); err != nil {
		return err
	}

	if writeToDatabase {
		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = targetMsTimestamp
		s.storage.SetSnapshotInfo(snapshotInfo)
		s.tangle.Events.SnapshotMilestoneIndexChanged.Trigger(targetIndex)
	}

	s.log.Infof("created %s snapshot for target index %d, took %v", snapshotNames[snapshotType], targetIndex, time.Since(ts))
	return nil
}

// returns an output consumer storing them into the database.
func newOutputConsumer(utxoManager *utxo.Manager) OutputConsumerFunc {
	return func(output *Output) error {
		switch addr := output.Address.(type) {
		case *iotago.WOTSAddress:
			return iotago.ErrWOTSNotImplemented
		case *iotago.Ed25519Address:

			outputID := iotago.UTXOInputID(output.OutputID)
			messageID := hornet.MessageID(output.MessageID)

			return utxoManager.AddUnspentOutput(utxo.GetOutput(&outputID, &messageID, addr, output.Amount))
		default:
			return iotago.ErrUnknownAddrType
		}
	}
}

// returns a function which calls the corresponding address type callback function with
// the origin argument and type casted address.
func callbackPerAddress(
	wotsAddrF func(interface{}, *iotago.WOTSAddress) error,
	edAddrF func(interface{}, *iotago.Ed25519Address) error) func(interface{}, iotago.Serializable) error {
	return func(obj interface{}, addr iotago.Serializable) error {
		switch a := addr.(type) {
		case *iotago.WOTSAddress:
			return wotsAddrF(obj, a)
		case *iotago.Ed25519Address:
			return edAddrF(obj, a)
		default:
			return iotago.ErrUnknownAddrType
		}
	}
}

// returns an iota.ErrWOTSNotImplemented error if called.
func errorOnWOTSAddr(_ interface{}, _ *iotago.WOTSAddress) error {
	return iotago.ErrWOTSNotImplemented
}

// creates a milestone diff consumer storing them into the database.
// if the ledger index within the database equals the produced milestone diff's index,
// then its changes are roll-backed, otherwise, if the index is higher than the ledger index,
// its mutations are applied on top of the latest state.
// the caller needs to make sure to set the ledger index accordingly beforehand.
func newMsDiffConsumer(utxoManager *utxo.Manager) MilestoneDiffConsumerFunc {
	return func(msDiff *MilestoneDiff) error {
		var newOutputs []*utxo.Output
		var newSpents []*utxo.Spent

		createdOutputAggr := callbackPerAddress(errorOnWOTSAddr, func(obj interface{}, addr *iotago.Ed25519Address) error {
			output := obj.(*Output)
			outputID := iotago.UTXOInputID(output.OutputID)
			messageID := hornet.MessageID(output.MessageID)
			newOutputs = append(newOutputs, utxo.GetOutput(&outputID, &messageID, addr, output.Amount))
			return nil
		})

		for _, output := range msDiff.Created {
			if err := createdOutputAggr(output, output.Address); err != nil {
				return err
			}
		}

		spentOutputAggr := callbackPerAddress(errorOnWOTSAddr, func(obj interface{}, addr *iotago.Ed25519Address) error {
			spent := obj.(*Spent)
			outputID := iotago.UTXOInputID(spent.OutputID)
			messageID := hornet.MessageID(spent.MessageID)
			newSpents = append(newSpents, utxo.NewSpent(utxo.GetOutput(&outputID, &messageID, addr, spent.Amount), &spent.TargetTransactionID, msDiff.MilestoneIndex))
			return nil
		})

		for _, spent := range msDiff.Consumed {
			if err := spentOutputAggr(spent, spent.Address); err != nil {
				return err
			}
		}

		ledgerIndex, err := utxoManager.ReadLedgerIndex()
		if err != nil {
			return err
		}

		switch {
		case ledgerIndex == msDiff.MilestoneIndex:
			return utxoManager.RollbackConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		case ledgerIndex+1 == msDiff.MilestoneIndex:
			return utxoManager.ApplyConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		default:
			return ErrWrongMilestoneDiffIndex
		}
	}
}

// returns a file header consumer, which stores the ledger milestone index up on execution in the database.
// the given targetHeader is populated with the value of the read file header.
func newFileHeaderConsumer(logger *logger.Logger, utxoManager *utxo.Manager, wantedNetworkID uint64, wantedType Type, targetHeader *ReadFileHeader) HeaderConsumerFunc {
	return func(header *ReadFileHeader) error {
		if header.Version != SupportedFormatVersion {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file version is %d but this HORNET version only supports %v", header.Version, SupportedFormatVersion)
		}

		if header.Type != wantedType {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file is of type %s but expected was %s", snapshotNames[header.Type], snapshotNames[wantedType])
		}

		if header.NetworkID != wantedNetworkID {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file network ID is %d but this HORNET is meant for %d", header.NetworkID, wantedNetworkID)
		}

		*targetHeader = *header
		logger.Infof("solid entry points: %d, outputs: %d, ms diffs: %d", header.SEPCount, header.OutputCount, header.MilestoneDiffCount)

		if err := utxoManager.StoreLedgerIndex(header.LedgerMilestoneIndex); err != nil {
			return err
		}

		return nil
	}
}

// returns a solid entry point consumer which stores them into the database.
// the SEPs are stored with the corresponding SEP milestone index from the snapshot.
func newSEPsConsumer(storage *storage.Storage, header *ReadFileHeader) SEPConsumerFunc {
	// note that we only get the hash of the SEP message instead
	// of also its associated oldest cone root index, since the index
	// of the snapshot milestone will be below max depth anyway.
	// this information was included in pre Chrysalis Phase 2 snapshots
	// but has been deemed unnecessary for the reason mentioned above.
	return func(solidEntryPointMessageID *hornet.MessageID) error {
		storage.SolidEntryPointsAdd(solidEntryPointMessageID, header.SEPMilestoneIndex)
		return nil
	}
}

// LoadSnapshotFromFile loads a snapshot file from the given file path into the storage.
func (s *Snapshot) LoadSnapshotFromFile(snapshotType Type, networkID uint64, filePath string) error {
	s.log.Infof("importing %s snapshot file...", snapshotNames[snapshotType])
	ts := time.Now()

	s.storage.WriteLockSolidEntryPoints()
	s.storage.ResetSolidEntryPoints()
	defer s.storage.WriteUnlockSolidEntryPoints()
	defer s.storage.StoreSolidEntryPoints()

	lsFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open %s snapshot file for import: %w", snapshotNames[snapshotType], err)
	}
	defer lsFile.Close()

	header := &ReadFileHeader{}
	headerConsumer := newFileHeaderConsumer(s.log, s.utxo, networkID, snapshotType, header)
	sepConsumer := newSEPsConsumer(s.storage, header)
	var outputConsumer OutputConsumerFunc
	if snapshotType == Full {
		outputConsumer = newOutputConsumer(s.utxo)
	}
	msDiffConsumer := newMsDiffConsumer(s.utxo)
	if err := StreamSnapshotDataFrom(lsFile, headerConsumer, sepConsumer, outputConsumer, msDiffConsumer); err != nil {
		return fmt.Errorf("unable to import %s snapshot file: %w", snapshotNames[snapshotType], err)
	}

	s.log.Infof("imported %s snapshot file, took %v", snapshotNames[snapshotType], time.Since(ts))
	if err := s.utxo.CheckLedgerState(); err != nil {
		return err
	}

	ledgerIndex, err := s.utxo.ReadLedgerIndex()
	if err != nil {
		return err
	}

	if ledgerIndex != header.SEPMilestoneIndex {
		return errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchSEPIndex, "%d != %d", ledgerIndex, header.SEPMilestoneIndex)
	}

	s.storage.SetSnapshotMilestone(header.NetworkID, header.SEPMilestoneIndex, header.SEPMilestoneIndex, header.SEPMilestoneIndex, time.Now())
	s.storage.SetSolidMilestoneIndex(header.SEPMilestoneIndex, false)

	return nil
}

// HandleNewSolidMilestoneEvent handles new solid milestone events which may trigger a delta snapshot creation and pruning.
func (s *Snapshot) HandleNewSolidMilestoneEvent(solidMilestoneIndex milestone.Index, shutdownSignal <-chan struct{}) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	if s.shouldTakeSnapshot(solidMilestoneIndex) {
		if err := s.createSnapshotWithoutLocking(Delta, solidMilestoneIndex-s.snapshotDepth, s.snapshotDeltaPath, true, shutdownSignal); err != nil {
			if errors.Is(err, ErrCritical) {
				s.log.Panicf("%s %s", ErrSnapshotCreationFailed, err)
			}
			s.log.Warnf("%s %s", ErrSnapshotCreationFailed, err)
		}
	}

	if !s.pruningEnabled {
		return
	}

	if solidMilestoneIndex <= s.pruningDelay {
		// not enough history
		return
	}

	if _, err := s.pruneDatabase(solidMilestoneIndex-s.pruningDelay, shutdownSignal); err != nil {
		s.log.Debugf("pruning aborted: %v", err)
	}
}
