package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
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
	// Returned when a delta snapshot is available, but no full snapshot is found.
	ErrInvalidSnapshotAvailabilityState = errors.New("invalid snapshot files availability")

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

type snapshotAvailability byte

const (
	snapshotAvailBoth snapshotAvailability = iota
	snapshotAvailOnlyFull
	snapshotAvailNone
)

type solidEntryPoint struct {
	messageID hornet.MessageID
	index     milestone.Index
}

// Snapshot handles reading and writing snapshot data.
type Snapshot struct {
	shutdownCtx                         context.Context
	log                                 *logger.Logger
	storage                             *storage.Storage
	utxo                                *utxo.Manager
	networkID                           uint64
	networkIDSource                     string
	snapshotFullPath                    string
	snapshotDeltaPath                   string
	downloadTargets                     []DownloadTarget
	solidEntryPointCheckThresholdPast   milestone.Index
	solidEntryPointCheckThresholdFuture milestone.Index
	additionalPruningThreshold          milestone.Index
	snapshotDepth                       milestone.Index
	snapshotInterval                    milestone.Index
	pruningEnabled                      bool
	pruningDelay                        milestone.Index
	pruneReceipts                       bool

	snapshotLock   syncutils.Mutex
	statusLock     syncutils.RWMutex
	isSnapshotting bool
	isPruning      bool

	Events *Events
}

// New creates a new snapshot instance.
func New(shutdownCtx context.Context,
	log *logger.Logger,
	storage *storage.Storage,
	utxo *utxo.Manager,
	networkID uint64,
	networkIDSource string,
	snapshotFullPath string,
	snapshotDeltaPath string,
	downloadTargets []DownloadTarget,
	solidEntryPointCheckThresholdPast milestone.Index,
	solidEntryPointCheckThresholdFuture milestone.Index,
	additionalPruningThreshold milestone.Index,
	snapshotDepth milestone.Index,
	snapshotInterval milestone.Index,
	pruningEnabled bool,
	pruningDelay milestone.Index,
	pruneReceipts bool) *Snapshot {

	return &Snapshot{
		shutdownCtx:                         shutdownCtx,
		log:                                 log,
		storage:                             storage,
		utxo:                                utxo,
		networkID:                           networkID,
		networkIDSource:                     networkIDSource,
		snapshotFullPath:                    snapshotFullPath,
		snapshotDeltaPath:                   snapshotDeltaPath,
		downloadTargets:                     downloadTargets,
		solidEntryPointCheckThresholdPast:   solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture: solidEntryPointCheckThresholdFuture,
		additionalPruningThreshold:          additionalPruningThreshold,
		snapshotDepth:                       snapshotDepth,
		snapshotInterval:                    snapshotInterval,
		pruningEnabled:                      pruningEnabled,
		pruningDelay:                        pruningDelay,
		pruneReceipts:                       pruneReceipts,
		Events: &Events{
			SnapshotMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
			SnapshotMetricsUpdated:        events.NewEvent(SnapshotMetricsCaller),
			PruningMilestoneIndexChanged:  events.NewEvent(milestone.IndexCaller),
			PruningMetricsUpdated:         events.NewEvent(PruningMetricsCaller),
		},
	}
}

func (s *Snapshot) IsSnapshottingOrPruning() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.isSnapshotting || s.isPruning
}

func (s *Snapshot) shouldTakeSnapshot(confirmedMilestoneIndex milestone.Index) bool {

	snapshotInfo := s.storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		s.log.Panic("No snapshotInfo found!")
	}

	if (confirmedMilestoneIndex < s.snapshotDepth+s.snapshotInterval) || (confirmedMilestoneIndex-s.snapshotDepth) < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return confirmedMilestoneIndex-(s.snapshotDepth+s.snapshotInterval) >= snapshotInfo.SnapshotIndex
}

func (s *Snapshot) forEachSolidEntryPoint(targetIndex milestone.Index, abortSignal <-chan struct{}, solidEntryPointConsumer func(sep *solidEntryPoint) bool) error {

	solidEntryPoints := make(map[string]milestone.Index)

	metadataMemcache := storage.NewMetadataMemcache(s.storage)
	defer metadataMemcache.Cleanup(true)

	// we share the same traverser for all milestones, so we don't cleanup the cachedMessages in between.
	// we don't need to call cleanup at the end, because we passed our own metadataMemcache.
	parentsTraverser := dag.NewParentTraverser(s.storage, abortSignal, metadataMemcache)

	// isSolidEntryPoint checks whether any direct child of the given message was referenced by a milestone which is above the target milestone.
	isSolidEntryPoint := func(messageID hornet.MessageID, targetIndex milestone.Index) bool {
		for _, childMessageID := range s.storage.GetChildrenMessageIDs(messageID) {
			cachedMsgMeta := metadataMemcache.GetCachedMetadataOrNil(childMessageID) // meta +1
			if cachedMsgMeta == nil {
				// Ignore this message since it doesn't exist anymore
				s.log.Warnf("%s, msg ID: %v, child msg ID: %v", ErrChildMsgNotFound, messageID.ToHex(), childMessageID.ToHex())
				continue
			}

			if referenced, at := cachedMsgMeta.GetMetadata().GetReferenced(); referenced && (at > targetIndex) {
				// referenced by a later milestone than targetIndex => solidEntryPoint
				return true
			}
		}
		return false
	}

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

		// traverse the milestone and collect all messages that were referenced by this milestone or newer
		if err := parentsTraverser.Traverse(hornet.MessageIDs{milestoneMessageID},
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

				select {
				case <-abortSignal:
					return ErrSnapshotCreationWasAborted
				default:
				}

				messageID := cachedMsgMeta.GetMetadata().GetMessageID()

				if isEntryPoint := isSolidEntryPoint(messageID, targetIndex); !isEntryPoint {
					return nil
				}

				referenced, at := cachedMsgMeta.GetMetadata().GetReferenced()
				if !referenced {
					return errors.Wrapf(ErrCritical, "solid entry point (%v) not referenced!", messageID.ToHex())
				}

				messageIDMapKey := messageID.ToMapKey()
				if _, exists := solidEntryPoints[messageIDMapKey]; !exists {
					solidEntryPoints[messageIDMapKey] = at
					if !solidEntryPointConsumer(&solidEntryPoint{messageID: messageID, index: at}) {
						return ErrSnapshotCreationWasAborted
					}
				}

				return nil
			},
			// called on missing parents
			// return error on missing parents
			nil,
			// called on solid entry points
			// Ignore solid entry points (snapshot milestone included)
			nil,
			// the pruning target index is also a solid entry point => traverse it anyways
			true); err != nil {
			if err == common.ErrOperationAborted {
				return ErrSnapshotCreationWasAborted
			}
		}
	}

	return nil
}

func (s *Snapshot) checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *storage.SnapshotInfo, checkSnapshotIndex bool) error {

	confirmedMilestoneIndex := s.storage.GetConfirmedMilestoneIndex()

	if confirmedMilestoneIndex < s.solidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum confirmed index: %d, actual confirmed index: %d", s.solidEntryPointCheckThresholdFuture+1, confirmedMilestoneIndex)
	}

	minimumIndex := s.solidEntryPointCheckThresholdPast + 1
	maximumIndex := confirmedMilestoneIndex - s.solidEntryPointCheckThresholdFuture

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
	return func() (hornet.MessageID, error) {
		obj, err := binder()
		if obj == nil || err != nil {
			return nil, err
		}
		return obj.(hornet.MessageID), nil
	}
}

// returns a producer which produces unspent outputs which exist for the current confirmed milestone.
func newCMIUTXOProducer(utxoManager *utxo.Manager) OutputProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		if err := utxoManager.ForEachUnspentOutput(func(output *utxo.Output) bool {
			prodChan <- &Output{MessageID: output.MessageID().ToArray(), OutputID: *output.OutputID(), OutputType: output.OutputType(), Address: output.Address(), Amount: output.Amount()}
			return true
		}, utxo.ReadLockLedger(false)); err != nil {
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
func newMsDiffsProducerDeltaFileAndDatabase(snapshotDeltaPath string, storage *storage.Storage, utxoManager *utxo.Manager, ledgerIndex milestone.Index, targetIndex milestone.Index) (MilestoneDiffProducerFunc, error) {
	prevDeltaFileMsDiffsProducer, err := newMsDiffsFromPreviousDeltaSnapshot(snapshotDeltaPath, ledgerIndex)
	if err != nil {
		return nil, err
	}

	var prevDeltaMsDiffProducerFinished bool
	var prevDeltaUpToIndex milestone.Index
	var dbMsDiffProducer MilestoneDiffProducerFunc
	mrf := MilestoneRetrieverFromStorage(storage)
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
			prevDeltaUpToIndex = milestone.Index(msDiff.Milestone.Index)
			return msDiff, nil
		}

		// TODO: check whether previous snapshot already hit the target index?

		prevDeltaMsDiffProducerFinished = true
		dbMsDiffProducer = newMsDiffsProducer(mrf, utxoManager, MsDiffDirectionOnwards, prevDeltaUpToIndex, targetIndex)
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
			func(id hornet.MessageID) error {
				// we don't care about solid entry points
				return nil
			}, nil, nil,
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

// MilestoneRetrieverFunc is a function which returns the milestone for the given index.
type MilestoneRetrieverFunc func(index milestone.Index) *iotago.Milestone

// MilestoneRetrieverFromStorage creates a MilestoneRetrieverFunc which access the storage.
// If it can not retrieve a wanted milestone it panics.
func MilestoneRetrieverFromStorage(storage *storage.Storage) MilestoneRetrieverFunc {
	return func(index milestone.Index) *iotago.Milestone {
		cachedMsMsg := storage.GetMilestoneCachedMessageOrNil(index)
		if cachedMsMsg == nil {
			panic(fmt.Sprintf("message for milestone with index %d is not stored in the database", index))
		}
		defer cachedMsMsg.Release()
		return cachedMsMsg.GetMessage().GetMilestone()
	}
}

// returns a producer which produces milestone diffs from/to with the given direction.
func newMsDiffsProducer(mrf MilestoneRetrieverFunc, utxoManager *utxo.Manager, direction MsDiffDirection, ledgerMilestoneIndex milestone.Index, targetIndex milestone.Index) MilestoneDiffProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		msIndexIterator := newMsIndexIterator(direction, ledgerMilestoneIndex, targetIndex)

		var done bool
		var msIndex milestone.Index

		for msIndex, done = msIndexIterator(); !done; msIndex, done = msIndexIterator() {
			diff, err := utxoManager.GetMilestoneDiffWithoutLocking(msIndex)
			if err != nil {
				errChan <- err
				close(prodChan)
				close(errChan)
				return
			}

			createdOutputs := make([]*Output, len(diff.Outputs))
			consumedOutputs := make([]*Spent, len(diff.Spents))

			for i, output := range diff.Outputs {
				createdOutputs[i] = &Output{
					MessageID:  output.MessageID().ToArray(),
					OutputID:   *output.OutputID(),
					OutputType: output.OutputType(),
					Address:    output.Address(),
					Amount:     output.Amount(),
				}
			}

			for i, spent := range diff.Spents {
				consumedOutputs[i] = &Spent{
					Output: Output{
						MessageID:  spent.MessageID().ToArray(),
						OutputID:   *spent.OutputID(),
						OutputType: spent.OutputType(),
						Address:    spent.Address(),
						Amount:     spent.Amount(),
					},
					TargetTransactionID: *spent.TargetTransactionID(),
				}
			}

			ms := mrf(msIndex)
			if ms == nil {
				panic(fmt.Sprintf("message for milestone with index %d could not be retrieved", msIndex))
			}

			prodChan <- &MilestoneDiff{
				Milestone:           ms,
				Created:             createdOutputs,
				Consumed:            consumedOutputs,
				SpentTreasuryOutput: diff.SpentTreasuryOutput,
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
func (s *Snapshot) readLedgerIndex() (milestone.Index, error) {
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

	// note that a full snapshot contains the ledger to the CMI of the node which generated it,
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

	timeStart := time.Now()

	s.utxo.ReadLockLedger()
	defer s.utxo.ReadUnlockLedger()

	timeReadLockLedger := time.Now()

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
		// ledger index corresponds to the CMI
		header.LedgerMilestoneIndex, err = s.readLedgerIndex()
		if err != nil {
			return err
		}

		// read out treasury tx
		header.TreasuryOutput, err = s.utxo.UnspentTreasuryOutputWithoutLocking()
		if err != nil {
			return err
		}

		// a full snapshot contains the ledger UTXOs as of the CMI
		// and the milestone diffs from the CMI back to the target index (excluding the target index)
		utxoProducer = newCMIUTXOProducer(s.utxo)
		milestoneDiffProducer = newMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxo, MsDiffDirectionBackwards, header.LedgerMilestoneIndex, targetIndex)

	case Delta:
		// ledger index corresponds to the origin snapshot snapshot ledger.
		// this will return an error if the full snapshot file is not available
		header.LedgerMilestoneIndex, err = s.readSnapshotIndexFromFullSnapshotFile()
		if err != nil {
			return err
		}

		_, err := os.Stat(filePath)
		deltaSnapshotFileExists := !os.IsNotExist(err)

		// a delta snapshot contains the milestone diffs from a full snapshot's snapshot index onwards
		switch {
		case snapshotInfo.SnapshotIndex == snapshotInfo.PruningIndex && !deltaSnapshotFileExists:
			// when booting up the first time on a full snapshot or in combination with a delta
			// snapshot, this indices will be the same. however, if we have a delta snapshot, we use it
			// since we might not have the actual milestone data.
			fallthrough
		case snapshotInfo.PruningIndex < header.LedgerMilestoneIndex:
			// we have the needed milestone diffs in the database
			milestoneDiffProducer = newMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxo, MsDiffDirectionOnwards, header.LedgerMilestoneIndex, targetIndex)
		default:
			// as the needed milestone diffs are pruned from the database, we need to use
			// the previous delta snapshot file to extract those in conjunction with what the database has available
			milestoneDiffProducer, err = newMsDiffsProducerDeltaFileAndDatabase(s.snapshotDeltaPath, s.storage, s.utxo, header.LedgerMilestoneIndex, targetIndex)
			if err != nil {
				return err
			}
		}
	}

	timeInit := time.Now()

	// stream data into snapshot file
	err, snapshotMetrics := StreamSnapshotDataTo(snapshotFile, uint64(ts.Unix()), header, newSEPsProducer(s, targetIndex, abortSignal), utxoProducer, milestoneDiffProducer)
	if err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[snapshotType], err)
	}

	timeStreamSnapshotData := time.Now()

	// finalize file
	if err := s.renameTempFile(snapshotFile, tempFilePath, filePath); err != nil {
		return err
	}

	timeSetSnapshotInfo := timeStreamSnapshotData
	timeSnapshotMilestoneIndexChanged := timeStreamSnapshotData
	if writeToDatabase {
		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = targetMsTimestamp
		s.storage.SetSnapshotInfo(snapshotInfo)
		timeSetSnapshotInfo = time.Now()
		s.Events.SnapshotMilestoneIndexChanged.Trigger(targetIndex)
		timeSnapshotMilestoneIndexChanged = time.Now()
	}

	snapshotMetrics.DurationReadLockLedger = timeReadLockLedger.Sub(timeStart)
	snapshotMetrics.DurationInit = timeInit.Sub(timeReadLockLedger)
	snapshotMetrics.DurationSetSnapshotInfo = timeSetSnapshotInfo.Sub(timeStreamSnapshotData)
	snapshotMetrics.DurationSnapshotMilestoneIndexChanged = timeSnapshotMilestoneIndexChanged.Sub(timeSetSnapshotInfo)
	snapshotMetrics.DurationTotal = time.Since(timeStart)

	s.Events.SnapshotMetricsUpdated.Trigger(snapshotMetrics)

	s.log.Infof("created %s snapshot for target index %d, took %v", snapshotNames[snapshotType], targetIndex, time.Since(ts).Truncate(time.Millisecond))
	return nil
}

// returns an output consumer storing them into the database.
func newOutputConsumer(utxoManager *utxo.Manager) OutputConsumerFunc {
	return func(output *Output) error {
		switch addr := output.Address.(type) {
		case *iotago.Ed25519Address:

			outputID := iotago.UTXOInputID(output.OutputID)
			messageID := hornet.MessageIDFromArray(output.MessageID)

			return utxoManager.AddUnspentOutput(utxo.CreateOutput(&outputID, messageID, output.OutputType, addr, output.Amount))
		default:
			return iotago.ErrUnknownAddrType
		}
	}
}

// returns a treasury output consumer which overrides an existing unspent treasury output with the new one.
func newUnspentTreasuryOutputConsumer(utxoManager *utxo.Manager) UnspentTreasuryOutputConsumerFunc {
	// leave like this for now in case we need to do more in the future
	return utxoManager.StoreUnspentTreasuryOutput
}

// returns a function which calls the corresponding address type callback function with
// the origin argument and type casted address.
func callbackPerAddress(
	edAddrF func(interface{}, *iotago.Ed25519Address) error) func(interface{}, iotago.Serializable) error {
	return func(obj interface{}, addr iotago.Serializable) error {
		switch a := addr.(type) {
		case *iotago.Ed25519Address:
			return edAddrF(obj, a)
		default:
			return iotago.ErrUnknownAddrType
		}
	}
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

		createdOutputAggr := callbackPerAddress(func(obj interface{}, addr *iotago.Ed25519Address) error {
			output := obj.(*Output)
			outputID := iotago.UTXOInputID(output.OutputID)
			messageID := hornet.MessageIDFromArray(output.MessageID)
			newOutputs = append(newOutputs, utxo.CreateOutput(&outputID, messageID, output.OutputType, addr, output.Amount))
			return nil
		})

		for _, output := range msDiff.Created {
			if err := createdOutputAggr(output, output.Address); err != nil {
				return err
			}
		}

		msIndex := milestone.Index(msDiff.Milestone.Index)
		spentOutputAggr := callbackPerAddress(func(obj interface{}, addr *iotago.Ed25519Address) error {

			spent := obj.(*Spent)
			outputID := iotago.UTXOInputID(spent.OutputID)
			messageID := hornet.MessageIDFromArray(spent.MessageID)
			newSpents = append(newSpents, utxo.NewSpent(utxo.CreateOutput(&outputID, messageID, spent.OutputType, addr, spent.Amount), &spent.TargetTransactionID, msIndex))
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

		var treasuryMut *utxo.TreasuryMutationTuple
		var rt *utxo.ReceiptTuple
		if treasuryOutput := msDiff.TreasuryOutput(); treasuryOutput != nil {
			treasuryMut = &utxo.TreasuryMutationTuple{
				NewOutput:   treasuryOutput,
				SpentOutput: msDiff.SpentTreasuryOutput,
			}
			rt = &utxo.ReceiptTuple{
				Receipt:        msDiff.Milestone.Receipt.(*iotago.Receipt),
				MilestoneIndex: msIndex,
			}
		}

		switch {
		case ledgerIndex == msIndex:
			return utxoManager.RollbackConfirmation(msIndex, newOutputs, newSpents, treasuryMut, rt)
		case ledgerIndex+1 == msIndex:
			return utxoManager.ApplyConfirmation(msIndex, newOutputs, newSpents, treasuryMut, rt)
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
	return func(solidEntryPointMessageID hornet.MessageID) error {
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
	var treasuryOutputConsumer UnspentTreasuryOutputConsumerFunc
	if snapshotType == Full {
		outputConsumer = newOutputConsumer(s.utxo)
		treasuryOutputConsumer = newUnspentTreasuryOutputConsumer(s.utxo)
	}
	msDiffConsumer := newMsDiffConsumer(s.utxo)
	if err := StreamSnapshotDataFrom(lsFile, headerConsumer, sepConsumer, outputConsumer, treasuryOutputConsumer, msDiffConsumer); err != nil {
		return fmt.Errorf("unable to import %s snapshot file: %w", snapshotNames[snapshotType], err)
	}

	s.log.Infof("imported %s snapshot file, took %v", snapshotNames[snapshotType], time.Since(ts).Truncate(time.Millisecond))

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
	s.storage.SetConfirmedMilestoneIndex(header.SEPMilestoneIndex, false)

	return nil
}

// HandleNewConfirmedMilestoneEvent handles new confirmed milestone events which may trigger a delta snapshot creation and pruning.
func (s *Snapshot) HandleNewConfirmedMilestoneEvent(confirmedMilestoneIndex milestone.Index, shutdownSignal <-chan struct{}) {
	if !s.storage.IsNodeAlmostSynced() {
		// do not prune or create snapshots while we are not synced
		return
	}

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	if s.shouldTakeSnapshot(confirmedMilestoneIndex) {
		if err := s.createSnapshotWithoutLocking(Delta, confirmedMilestoneIndex-s.snapshotDepth, s.snapshotDeltaPath, true, shutdownSignal); err != nil {
			if errors.Is(err, ErrCritical) {
				s.log.Panicf("%s %s", ErrSnapshotCreationFailed, err)
			}
			s.log.Warnf("%s %s", ErrSnapshotCreationFailed, err)
		}
	}

	if !s.pruningEnabled {
		return
	}

	if confirmedMilestoneIndex <= s.pruningDelay {
		// not enough history
		return
	}

	if _, err := s.pruneDatabase(confirmedMilestoneIndex-s.pruningDelay, shutdownSignal); err != nil {
		s.log.Debugf("pruning aborted: %v", err)
	}
}

// ImportSnapshots imports snapshot data from the configured file paths.
// automatically downloads snapshot data if no files are available.
func (s *Snapshot) ImportSnapshots() error {
	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return err
	}

	if snapAvail == snapshotAvailNone {
		if err := s.downloadSnapshotFiles(s.snapshotFullPath, s.snapshotDeltaPath); err != nil {
			return err
		}
	}

	if err := s.LoadSnapshotFromFile(Full, s.networkID, s.snapshotFullPath); err != nil {
		s.storage.MarkDatabaseCorrupted()
		return err
	}

	if snapAvail == snapshotAvailOnlyFull {
		return nil
	}

	if err := s.LoadSnapshotFromFile(Delta, s.networkID, s.snapshotDeltaPath); err != nil {
		s.storage.MarkDatabaseCorrupted()
		return err
	}

	return nil
}

// checks that either both snapshot files are available, only the full snapshot or none.
func (s *Snapshot) checkSnapshotFilesAvailability(fullPath string, deltaPath string) (snapshotAvailability, error) {
	switch {
	case len(fullPath) == 0:
		return 0, fmt.Errorf("%w: full snapshot file path not defined", ErrNoSnapshotSpecified)
	case len(deltaPath) == 0:
		return 0, fmt.Errorf("%w: delta snapshot file path not defined", ErrNoSnapshotSpecified)
	}

	_, fullSnapshotStatErr := os.Stat(fullPath)
	_, deltaSnapshotStatErr := os.Stat(deltaPath)

	switch {
	case os.IsNotExist(fullSnapshotStatErr) && deltaSnapshotStatErr == nil:
		// only having the delta snapshot file does not make sense,
		// as it relies on a full snapshot file to be available.
		// downloading the full snapshot would not help, as it will probably
		// be incompatible with the delta snapshot index.
		return 0, fmt.Errorf("%w: there exists a delta snapshot but not a full snapshot file, delete the delta snapshot file and restart", ErrInvalidSnapshotAvailabilityState)
	case os.IsNotExist(fullSnapshotStatErr) && os.IsNotExist(deltaSnapshotStatErr):
		return snapshotAvailNone, nil
	case fullSnapshotStatErr == nil && os.IsNotExist(deltaSnapshotStatErr):
		return snapshotAvailOnlyFull, nil
	default:
		return snapshotAvailBoth, nil
	}
}

// ensures that the folders to both paths exists and then downloads the appropriate snapshot files.
func (s *Snapshot) downloadSnapshotFiles(fullPath string, deltaPath string) error {
	fullPathDir := filepath.Dir(fullPath)
	deltaPathDir := filepath.Dir(deltaPath)

	if err := os.MkdirAll(fullPathDir, 0700); err != nil {
		return fmt.Errorf("could not create snapshot dir '%s': %w", fullPath, err)
	}

	if err := os.MkdirAll(deltaPathDir, 0700); err != nil {
		return fmt.Errorf("could not create snapshot dir '%s': %w", fullPath, err)
	}

	if len(s.downloadTargets) == 0 {
		return ErrNoSnapshotDownloadURL
	}

	targetsJson, err := json.MarshalIndent(s.downloadTargets, "", "   ")
	if err != nil {
		return fmt.Errorf("unable to marshal targets into formatted JSON: %w", err)
	}
	s.log.Infof("downloading snapshot files from one of the provided sources %s", string(targetsJson))

	if err := s.DownloadSnapshotFiles(fullPath, deltaPath, s.downloadTargets); err != nil {
		return fmt.Errorf("unable to download snapshot files: %w", err)
	}

	s.log.Info("snapshot download finished")
	return nil
}

// checks that the current snapshot info is valid regarding its network ID and the ledger state.
func (s *Snapshot) CheckCurrentSnapshot(snapshotInfo *storage.SnapshotInfo) error {

	// check that the stored snapshot corresponds to the wanted network ID
	if snapshotInfo.NetworkID != s.networkID {
		s.log.Panicf("node is configured to operate in network %d/%s but the stored snapshot data corresponds to %d", s.networkID, s.networkIDSource, snapshotInfo.NetworkID)
	}

	// if we don't enforce loading of a snapshot,
	// we can check the ledger state of the current database and start the node.
	if err := s.utxo.CheckLedgerState(); err != nil {
		s.log.Fatal(err.Error())
	}

	return nil
}
