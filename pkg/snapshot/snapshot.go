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

	ErrNoSnapshotSpecified               = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL             = errors.New("no download URL given for local snapshot in config")
	ErrSnapshotDownloadWasAborted        = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource     = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotImportWasAborted          = errors.New("snapshot import was aborted")
	ErrSnapshotImportFailed              = errors.New("snapshot import failed")
	ErrSnapshotCreationWasAborted        = errors.New("operation was aborted")
	ErrSnapshotCreationFailed            = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew                 = errors.New("snapshot target is too new")
	ErrTargetIndexTooOld                 = errors.New("snapshot target is too old")
	ErrNotEnoughHistory                  = errors.New("not enough history")
	ErrNoPruningNeeded                   = errors.New("no pruning needed")
	ErrPruningAborted                    = errors.New("pruning was aborted")
	ErrUnreferencedTxInSubtangle         = errors.New("unreferenced msg in subtangle")
	ErrInvalidBalance                    = errors.New("invalid balance! total does not match supply")
	ErrWrongCoordinatorPublicKeyDatabase = errors.New("configured coordinator public key does not match database information")
)

type solidEntryPoint struct {
	messageID *hornet.MessageID
	index     milestone.Index
}

type Snapshot struct {
	shutdownCtx                         context.Context
	log                                 *logger.Logger
	storage                             *storage.Storage
	tangle                              *tangle.Tangle
	utxo                                *utxo.Manager
	snapshotPath                        string
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
	snapshotPath string,
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
		snapshotPath:                        snapshotPath,
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
			return (referenced && at >= milestoneIndex), nil
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

			if isEntryPoint := s.isSolidEntryPoint(parentMessageID, targetIndex); isEntryPoint {
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
	}

	return nil
}

func (s *Snapshot) checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *storage.SnapshotInfo, checkSnapshotIndex bool) error {

	solidMilestoneIndex := s.storage.GetSolidMilestoneIndex()

	if solidMilestoneIndex < s.solidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum solid index: %d, actual solid index: %d", s.solidEntryPointCheckThresholdFuture+1, solidMilestoneIndex)
	}

	minimumIndex := milestone.Index(s.solidEntryPointCheckThresholdPast + 1)
	maximumIndex := solidMilestoneIndex - s.solidEntryPointCheckThresholdFuture

	if checkSnapshotIndex && minimumIndex < snapshotInfo.SnapshotIndex+1 {
		minimumIndex = snapshotInfo.SnapshotIndex + 1
	}

	if minimumIndex < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		minimumIndex = snapshotInfo.PruningIndex + 1 + s.solidEntryPointCheckThresholdPast
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

func (s *Snapshot) setIsSnapshotting(value bool) {
	s.statusLock.Lock()
	s.isSnapshotting = value
	s.statusLock.Unlock()
}

func (s *Snapshot) CreateLocalSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createFullLocalSnapshotWithoutLocking(targetIndex, filePath, writeToDatabase, abortSignal)
}

func (s *Snapshot) createFullLocalSnapshotWithoutLocking(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	s.log.Infof("creating local snapshot for targetIndex %d", targetIndex)
	ts := time.Now()

	snapshotInfo := s.storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(ErrCritical, "no snapshot info found")
	}

	if err := s.checkSnapshotLimits(targetIndex, snapshotInfo, writeToDatabase); err != nil {
		return err
	}

	s.setIsSnapshotting(true)
	defer s.setIsSnapshotting(false)

	cachedTargetMilestone := s.storage.GetCachedMilestoneOrNil(targetIndex) // milestone +1
	if cachedTargetMilestone == nil {
		return errors.Wrapf(ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedTargetMilestone.Release(true) // milestone -1

	header := &FileHeader{
		Version:           SupportedFormatVersion,
		Type:              Full,
		NetworkID:         snapshotInfo.NetworkID,
		SEPMilestoneIndex: targetIndex,
	}

	// build temp file path
	filePathTmp := filePath + "_tmp"
	_ = os.Remove(filePathTmp)

	// create temp file
	lsFile, err := os.OpenFile(filePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create tmp local snapshot file: %w", err)
	}

	s.utxo.ReadLockLedger()
	defer s.utxo.ReadUnlockLedger()

	ledgerMilestoneIndex, err := s.utxo.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return fmt.Errorf("unable to read current ledger index: %w", err)
	}

	cachedMilestone := s.storage.GetCachedMilestoneOrNil(ledgerMilestoneIndex)
	if cachedMilestone == nil {
		return errors.Wrapf(ErrCritical, "milestone (%d) not found!", ledgerMilestoneIndex)
	}

	header.LedgerMilestoneIndex = ledgerMilestoneIndex

	cachedMilestone.Release(true)

	//
	// solid entry points
	//
	solidEntryPointProducerChan := make(chan *hornet.MessageID)
	solidEntryPointProducerErrorChan := make(chan error)

	solidEntryPointProducerFunc := func() (*hornet.MessageID, error) {
		select {
		case err, ok := <-solidEntryPointProducerErrorChan:
			if !ok {
				return nil, nil
			}
			return nil, err
		case solidEntryPointMessageID, ok := <-solidEntryPointProducerChan:
			if !ok {
				return nil, nil
			}
			return solidEntryPointMessageID, nil
		}
	}

	go func() {
		// calculate solid entry points for the target index
		if err := s.forEachSolidEntryPoint(targetIndex, abortSignal, func(sep *solidEntryPoint) bool {
			solidEntryPointProducerChan <- sep.messageID
			return true
		}); err != nil {
			solidEntryPointProducerErrorChan <- err
		}

		close(solidEntryPointProducerChan)
		close(solidEntryPointProducerErrorChan)
	}()

	//
	// unspent outputs
	//
	outputProducerChan := make(chan *Output)
	outputProducerErrorChan := make(chan error)

	outputProducerFunc := func() (*Output, error) {
		select {
		case err, ok := <-outputProducerErrorChan:
			if !ok {
				return nil, nil
			}
			return nil, err
		case output, ok := <-outputProducerChan:
			if !ok {
				return nil, nil
			}
			return output, nil
		}
	}

	go func() {
		if err := s.utxo.ForEachUnspentOutputWithoutLocking(func(output *utxo.Output) bool {
			outputProducerChan <- &Output{MessageID: *output.MessageID(), OutputID: *output.OutputID(), Address: output.Address(), Amount: output.Amount()}
			return true
		}); err != nil {
			outputProducerErrorChan <- err
		}

		close(outputProducerChan)
		close(outputProducerErrorChan)
	}()

	//
	// milestone diffs
	//
	milestoneDiffProducerChan := make(chan *MilestoneDiff)
	milestoneDiffProducerErrorChan := make(chan error)

	milestoneDiffProducerFunc := func() (*MilestoneDiff, error) {
		select {
		case err, ok := <-milestoneDiffProducerErrorChan:
			if !ok {
				return nil, nil
			}
			return nil, err
		case msDiff, ok := <-milestoneDiffProducerChan:
			if !ok {
				return nil, nil
			}
			return msDiff, nil
		}
	}

	go func() {
		// targetIndex should not be included in the snapshot, because we only need the diff of targetIndex+1 to calculate the ledger index of targetIndex
		for msIndex := ledgerMilestoneIndex; msIndex > targetIndex; msIndex-- {
			newOutputs, newSpents, err := s.utxo.GetMilestoneDiffsWithoutLocking(msIndex)
			if err != nil {
				milestoneDiffProducerErrorChan <- err
				close(milestoneDiffProducerChan)
				close(milestoneDiffProducerErrorChan)
				return
			}

			var createdOutputs []*Output
			var consumedOutputs []*Spent

			for _, output := range newOutputs {
				createdOutputs = append(createdOutputs, &Output{MessageID: *output.MessageID(), OutputID: *output.OutputID(), Address: output.Address(), Amount: output.Amount()})
			}

			for _, spent := range newSpents {
				consumedOutputs = append(consumedOutputs, &Spent{Output: Output{MessageID: *spent.MessageID(), OutputID: *spent.OutputID(), Address: spent.Address(), Amount: spent.Amount()}, TargetTransactionID: *spent.TargetTransactionID()})
			}

			milestoneDiffProducerChan <- &MilestoneDiff{MilestoneIndex: msIndex, Created: createdOutputs, Consumed: consumedOutputs}
		}

		close(milestoneDiffProducerChan)
		close(milestoneDiffProducerErrorChan)
	}()

	if err := StreamLocalSnapshotDataTo(lsFile, uint64(ts.Unix()), header, solidEntryPointProducerFunc, outputProducerFunc, milestoneDiffProducerFunc); err != nil {
		_ = lsFile.Close()
		return fmt.Errorf("couldn't generate local snapshot file: %w", err)
	}

	// rename tmp file to final file name
	if err := lsFile.Close(); err != nil {
		return fmt.Errorf("unable to close local snapshot file: %w", err)
	}
	if err := os.Rename(filePathTmp, filePath); err != nil {
		return fmt.Errorf("unable to rename temp local snapshot file: %w", err)
	}

	if writeToDatabase {
		/*
			// ToDo: Do we still store the initial snapshot in the database, or will the last full snapshot file be kept somewhere on disk?

			// This has to be done before acquiring the SolidEntryPoints Lock, otherwise there is a race condition with "solidifyMilestone"
			// In "solidifyMilestone" the LedgerLock is acquired, but by traversing the tangle, the SolidEntryPoint Lock is also acquired.
			// ToDo: we should flush the caches here, just to be sure that all information before this local snapshot we stored in the persistence layer.
			err = s.storage.StoreSnapshotBalancesInDatabase(newBalances, targetIndex)
			if err != nil {
				return errors.Wrap(ErrCritical, err.Error())
			}
		*/

		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = cachedTargetMilestone.GetMilestone().Timestamp
		s.storage.SetSnapshotInfo(snapshotInfo)

		s.tangle.Events.SnapshotMilestoneIndexChanged.Trigger(targetIndex)
	}

	s.log.Infof("created local snapshot for target index %d, took %v", targetIndex, time.Since(ts))
	return nil
}

func (s *Snapshot) LoadFullSnapshotFromFile(networkID uint64, filePath string) error {
	s.log.Info("importing full snapshot file...")
	ts := time.Now()

	lsFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open local snapshot file for import: %w", err)
	}
	defer lsFile.Close()

	var lsHeader *ReadFileHeader
	headerConsumer := func(header *ReadFileHeader) error {
		if header.Version != SupportedFormatVersion {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file version is %d but this HORNET version only supports %v", header.Version, SupportedFormatVersion)
		}
		if header.NetworkID != networkID {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file network ID is %d but this HORNET is meant for %d", header.NetworkID, networkID)
		}

		lsHeader = header
		s.log.Infof("solid entry points: %d, outputs: %d, ms diffs: %d", header.SEPCount, header.OutputCount, header.MilestoneDiffCount)

		if err := s.utxo.StoreLedgerIndex(lsHeader.LedgerMilestoneIndex); err != nil {
			return err
		}

		return nil
	}

	// note that we only get the hash of the SEP message instead
	// of also its associated oldest cone root index, since the index
	// of the local snapshot milestone will be below max depth anyway.
	// this information was included in pre Chrysalis Phase 2 local snapshots
	// but has been deemed unnecessary for the reason mentioned above.
	sepConsumer := func(solidEntryPointMessageID *hornet.MessageID) error {
		s.storage.SolidEntryPointsAdd(solidEntryPointMessageID, lsHeader.SEPMilestoneIndex)
		return nil
	}

	outputConsumer := func(output *Output) error {
		switch addr := output.Address.(type) {
		case *iotago.WOTSAddress:
			return iotago.ErrWOTSNotImplemented
		case *iotago.Ed25519Address:

			outputID := iotago.UTXOInputID(output.OutputID)
			messageID := hornet.MessageID(output.MessageID)

			return s.utxo.AddUnspentOutput(utxo.GetOutput(&outputID, &messageID, addr, output.Amount))
		default:
			return iotago.ErrUnknownAddrType
		}
	}

	msDiffConsumer := func(msDiff *MilestoneDiff) error {
		var newOutputs []*utxo.Output
		var newSpents []*utxo.Spent

		for _, output := range msDiff.Created {
			switch addr := output.Address.(type) {
			case *iotago.WOTSAddress:
				return iotago.ErrWOTSNotImplemented
			case *iotago.Ed25519Address:

				outputID := iotago.UTXOInputID(output.OutputID)
				messageID := hornet.MessageID(output.MessageID)

				newOutputs = append(newOutputs, utxo.GetOutput(&outputID, &messageID, addr, output.Amount))
			default:
				return iotago.ErrUnknownAddrType
			}
		}

		for _, spent := range msDiff.Consumed {
			switch addr := spent.Address.(type) {
			case *iotago.WOTSAddress:
				return iotago.ErrWOTSNotImplemented
			case *iotago.Ed25519Address:
				outputID := iotago.UTXOInputID(spent.OutputID)
				messageID := hornet.MessageID(spent.MessageID)

				newSpents = append(newSpents, utxo.NewSpent(utxo.GetOutput(&outputID, &messageID, addr, spent.Amount), &spent.TargetTransactionID, msDiff.MilestoneIndex))
			default:
				return iotago.ErrUnknownAddrType
			}
		}

		ledgerIndex, err := s.utxo.ReadLedgerIndex()
		if err != nil {
			return err
		}

		if ledgerIndex == msDiff.MilestoneIndex {
			return s.utxo.RollbackConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		}

		if ledgerIndex == msDiff.MilestoneIndex+1 {
			return s.utxo.ApplyConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		}

		return ErrWrongMilestoneDiffIndex
	}

	s.storage.WriteLockSolidEntryPoints()
	s.storage.ResetSolidEntryPoints()
	defer s.storage.WriteUnlockSolidEntryPoints()
	defer s.storage.StoreSolidEntryPoints()

	if err := StreamLocalSnapshotDataFrom(lsFile, headerConsumer, sepConsumer, outputConsumer, msDiffConsumer); err != nil {
		return fmt.Errorf("unable to import local snapshot file: %w", err)
	}

	s.log.Infof("imported local snapshot file, took %v", time.Since(ts))

	if err := s.utxo.CheckLedgerState(); err != nil {
		return err
	}

	ledgerIndex, err := s.utxo.ReadLedgerIndex()
	if err != nil {
		return err
	}

	if ledgerIndex != lsHeader.SEPMilestoneIndex {
		return errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchSEPIndex, "%d != %d", ledgerIndex, lsHeader.SEPMilestoneIndex)
	}

	s.storage.SetSnapshotMilestone(lsHeader.NetworkID, lsHeader.SEPMilestoneIndex, lsHeader.SEPMilestoneIndex, lsHeader.SEPMilestoneIndex, time.Now())
	s.storage.SetSolidMilestoneIndex(lsHeader.SEPMilestoneIndex, false)

	return nil
}

func (s *Snapshot) TriggerNewSolidMilestoneEvent(solidMilestoneIndex milestone.Index, shutdownSignal <-chan struct{}) {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	if s.shouldTakeSnapshot(solidMilestoneIndex) {
		if err := s.createFullLocalSnapshotWithoutLocking(solidMilestoneIndex-s.snapshotDepth, s.snapshotPath, true, shutdownSignal); err != nil {
			if errors.Is(err, ErrCritical) {
				s.log.Panicf("%s %s", ErrSnapshotCreationFailed, err)
			}
			s.log.Warnf("%s %s", ErrSnapshotCreationFailed, err)
		}
	}

	if s.pruningEnabled {
		if solidMilestoneIndex <= s.pruningDelay {
			// Not enough history
			return
		}

		if _, err := s.pruneDatabase(solidMilestoneIndex-s.pruningDelay, shutdownSignal); err != nil {
			s.log.Debugf("pruning aborted: %v", err)
		}
	}
}
