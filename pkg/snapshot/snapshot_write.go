package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/ioutils"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	coreDatabase "github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

// MsDiffDirection determines the milestone diff direction.
type MsDiffDirection byte

const (
	// MsDiffDirectionBackwards defines to produce milestone diffs in backwards direction.
	MsDiffDirectionBackwards MsDiffDirection = iota
	// MsDiffDirectionOnwards defines to produce milestone diffs in onwards direction.
	MsDiffDirectionOnwards
)

const (
	// AdditionalMilestoneDiffRange defines the maximum number of additional
	// milestone diffs that are stored in the full snapshot.
	// These are used to reconstruct pending protocol parameter updates.
	AdditionalMilestoneDiffRange syncmanager.MilestoneIndexDelta = 30
)

// MilestoneRetrieverFunc is a function which returns the milestone for the given index.
type MilestoneRetrieverFunc func(index iotago.MilestoneIndex) (*iotago.Milestone, error)

// MergeInfo holds information about a merge of a full and delta snapshot.
type MergeInfo struct {
	// The header of the full snapshot.
	FullSnapshotHeader *FullSnapshotHeader
	// The header of the delta snapshot.
	DeltaSnapshotHeader *DeltaSnapshotHeader
	// The header of the merged snapshot.
	MergedSnapshotHeader *FullSnapshotHeader
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

// NewSEPsProducer returns a producer which produces solid entry points.
func NewSEPsProducer(
	ctx context.Context,
	dbStorage *storage.Storage,
	targetIndex iotago.MilestoneIndex,
	solidEntryPointCheckThresholdPast iotago.MilestoneIndex) SEPProducerFunc {

	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		// calculate solid entry points for the target index
		if err := dag.ForEachSolidEntryPoint(
			ctx,
			dbStorage,
			targetIndex,
			solidEntryPointCheckThresholdPast,
			func(sep *storage.SolidEntryPoint) bool {
				prodChan <- sep.BlockID

				return true
			}); err != nil {
			if errors.Is(err, common.ErrOperationAborted) {
				errChan <- ErrSnapshotCreationWasAborted
			} else {
				errChan <- err
			}
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)

	return func() (iotago.BlockID, error) {
		obj, err := binder()
		if err != nil {
			return iotago.EmptyBlockID(), err
		}
		if obj == nil {
			return iotago.EmptyBlockID(), ErrNoMoreSEPToProduce
		}

		blockID, ok := obj.(iotago.BlockID)
		if !ok {
			return iotago.EmptyBlockID(), fmt.Errorf("expected iotago.BlockID, got %T", obj)
		}

		return blockID, nil
	}
}

// NewSEPsProducerFromMilestone returns a producer which produces the parents of a milestone as solid entry points.
func NewSEPsProducerFromMilestone(milestonePayload *iotago.Milestone) SEPProducerFunc {

	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		// use the parents of the milestone as solid entry points.
		for _, parent := range milestonePayload.Parents {
			prodChan <- parent
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)

	return func() (iotago.BlockID, error) {
		obj, err := binder()
		if err != nil {
			return iotago.EmptyBlockID(), err
		}
		if obj == nil {
			return iotago.EmptyBlockID(), ErrNoMoreSEPToProduce
		}

		blockID, ok := obj.(iotago.BlockID)
		if !ok {
			return iotago.EmptyBlockID(), fmt.Errorf("expected iotago.BlockID, got %T", obj)
		}

		return blockID, nil
	}
}

// NewCMIUTXOProducer returns a producer which produces unspent outputs which exist for the current confirmed milestone.
func NewCMIUTXOProducer(utxoManager *utxo.Manager) OutputProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		if err := utxoManager.ForEachUnspentOutput(func(output *utxo.Output) bool {
			prodChan <- output

			return true
		}, utxo.ReadLockLedger(false)); err != nil {
			errChan <- err
		}

		close(prodChan)
		close(errChan)
	}()

	binder := producerFromChannels(prodChan, errChan)

	return func() (*utxo.Output, error) {
		obj, err := binder()
		if obj == nil || err != nil {
			return nil, err
		}

		output, ok := obj.(*utxo.Output)
		if !ok {
			return nil, fmt.Errorf("expected *utxo.Output, got %T", obj)
		}

		return output, nil
	}
}

// NewMsIndexIterator returns an iterator producing milestone indices with the given direction from/to the milestone range.
func NewMsIndexIterator(direction MsDiffDirection, ledgerIndex iotago.MilestoneIndex, targetIndex iotago.MilestoneIndex) func() (msIndex iotago.MilestoneIndex, done bool) {
	var firstPassDone bool
	switch direction {
	case MsDiffDirectionOnwards:
		// we skip the diff of the ledger milestone
		msIndex := ledgerIndex + 1

		return func() (iotago.MilestoneIndex, bool) {
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

		return func() (iotago.MilestoneIndex, bool) {
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

// MilestoneRetrieverFromStorage creates a MilestoneRetrieverFunc which access the storage.
// If it can not retrieve a wanted milestone it panics.
func MilestoneRetrieverFromStorage(dbStorage *storage.Storage) MilestoneRetrieverFunc {
	return func(index iotago.MilestoneIndex) (*iotago.Milestone, error) {
		cachedMilestone := dbStorage.CachedMilestoneByIndexOrNil(index) // milestone +1
		if cachedMilestone == nil {
			return nil, fmt.Errorf("block for milestone with index %d is not stored in the database", index)
		}
		defer cachedMilestone.Release(true) // milestone -1

		return cachedMilestone.Milestone().Milestone(), nil
	}
}

// NewMsDiffsProducer returns a producer which produces milestone diffs from/to with the given direction.
func NewMsDiffsProducer(mrf MilestoneRetrieverFunc, utxoManager *utxo.Manager, direction MsDiffDirection, ledgerMilestoneIndex iotago.MilestoneIndex, targetIndex iotago.MilestoneIndex) MilestoneDiffProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		msIndexIterator := NewMsIndexIterator(direction, ledgerMilestoneIndex, targetIndex)

		var done bool
		var msIndex iotago.MilestoneIndex

		for msIndex, done = msIndexIterator(); !done; msIndex, done = msIndexIterator() {
			diff, err := utxoManager.MilestoneDiffWithoutLocking(msIndex)
			if err != nil {
				errChan <- err
				close(prodChan)
				close(errChan)

				return
			}

			milestonePayload, err := mrf(msIndex)
			if err != nil {
				errChan <- fmt.Errorf("block for milestone with index %d could not be retrieved: %w", msIndex, err)
				close(prodChan)
				close(errChan)

				return
			}
			if milestonePayload == nil {
				errChan <- fmt.Errorf("block for milestone with index %d could not be retrieved", msIndex)
				close(prodChan)
				close(errChan)

				return
			}

			prodChan <- &MilestoneDiff{
				Milestone:           milestonePayload,
				Created:             diff.Outputs,
				Consumed:            diff.Spents,
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

		msdiff, ok := obj.(*MilestoneDiff)
		if !ok {
			return nil, fmt.Errorf("expected *MilestoneDiff, got %T", obj)
		}

		return msdiff, nil
	}
}

// reads out the index of the milestone which currently represents the ledger state.
func (s *Manager) readLedgerIndex() (iotago.MilestoneIndex, error) {
	ledgerMilestoneIndex, err := s.utxoManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return 0, fmt.Errorf("unable to read current ledger index: %w", err)
	}

	if !s.storage.ContainsMilestoneIndex(ledgerMilestoneIndex) {
		return 0, errors.Wrapf(common.ErrCritical, "milestone (%d) not found", ledgerMilestoneIndex)
	}

	return ledgerMilestoneIndex, nil
}

// reads out the snapshot milestone index from the full snapshot file.
func (s *Manager) readSnapshotHeaderFromFullSnapshotFile() (*FullSnapshotHeader, error) {
	fullSnapshotHeader, err := ReadFullSnapshotHeaderFromFile(s.snapshotFullPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read full snapshot header for origin snapshot milestone index: %w", err)
	}

	// note that a full snapshot contains the ledger to the CMI of the node which generated it,
	// however, the state is rolled backed to the target index, therefore, the target index
	// is the actual point from which on the delta snapshot should contain milestone diffs
	return fullSnapshotHeader, nil
}

// creates a snapshot file by streaming data from the database into a snapshot file.
// the ledger needs to be read locked the whole time to stream the current ledger state into the full snapshot.
func (s *Manager) createFullSnapshotWithoutLocking(
	ctx context.Context,
	targetIndex iotago.MilestoneIndex,
	filePath string,
	writeToDatabase bool) error {

	s.LogInfof("creating %s snapshot for targetIndex %d", snapshotNames[Full], targetIndex)
	ts := time.Now()

	s.setIsSnapshotting(true)
	defer s.setIsSnapshotting(false)

	timeStart := time.Now()

	s.utxoManager.ReadLockLedger()
	defer s.utxoManager.ReadUnlockLedger()

	timeReadLockLedger := time.Now()

	if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		// do not create the snapshot if the node was shut down
		return err
	}

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(common.ErrCritical, common.ErrSnapshotInfoNotFound.Error())
	}

	if err := checkSnapshotLimits(
		snapshotInfo,
		s.syncManager.ConfirmedMilestoneIndex(),
		targetIndex,
		false,
		s.solidEntryPointCheckThresholdPast,
		s.solidEntryPointCheckThresholdFuture,
		// if we write the snapshot state to the database, the newly generated snapshot index must be greater than the last snapshot index
		writeToDatabase); err != nil {
		return err
	}

	cachedMilestoneTarget := s.storage.CachedMilestoneByIndexOrNil(targetIndex) // milestone +1
	if cachedMilestoneTarget == nil {
		return errors.Wrapf(common.ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedMilestoneTarget.Release(true) // milestone -1

	targetMilestoneTimestamp := cachedMilestoneTarget.Milestone().TimestampUnix()
	targetMilestoneID := cachedMilestoneTarget.Milestone().MilestoneID()

	// ledger index corresponds to the CMI
	ledgerIndex, err := s.readLedgerIndex()
	if err != nil {
		return err
	}

	// read out treasury tx
	unspentTreasuryOutput, err := s.utxoManager.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return fmt.Errorf("unable to get unspent treasury output: %w", err)
	}

	protoParamsMsOption, err := s.storage.ProtocolParametersMilestoneOption(ledgerIndex)
	if err != nil {
		return fmt.Errorf("loading protocol parameters milestone option failed: %w", err)
	}

	timeInit := time.Now()

	fullHeader := &FullSnapshotHeader{
		Version:                    SupportedFormatVersion,
		Type:                       Full,
		GenesisMilestoneIndex:      snapshotInfo.GenesisMilestoneIndex(),
		TargetMilestoneIndex:       targetIndex,
		TargetMilestoneTimestamp:   targetMilestoneTimestamp,
		TargetMilestoneID:          targetMilestoneID,
		LedgerMilestoneIndex:       ledgerIndex,
		TreasuryOutput:             unspentTreasuryOutput,
		ProtocolParamsMilestoneOpt: protoParamsMsOption,
		OutputCount:                0,
		MilestoneDiffCount:         0,
		SEPCount:                   0,
	}

	snapshotFile, tempFilePath, err := ioutils.CreateTempFile(filePath)
	if err != nil {
		return err
	}

	// a full snapshot contains the ledger UTXOs as of the CMI and the milestone diffs from
	// the CMI back to target index - AdditionalMilestoneDiffRange (excluding the last index)
	// the "AdditionalMilestoneDiffRange" milestone diffs are needed to reconstruct pending protocol parameter updates.
	utxoProducer := NewCMIUTXOProducer(s.utxoManager)
	milestoneDiffProducer := NewMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxoManager, MsDiffDirectionBackwards, fullHeader.LedgerMilestoneIndex, targetIndex-AdditionalMilestoneDiffRange)
	sepProducer := NewSEPsProducer(ctx, s.storage, targetIndex, s.solidEntryPointCheckThresholdPast)

	// stream data into snapshot file
	snapshotMetrics, err := StreamFullSnapshotDataTo(snapshotFile, fullHeader, utxoProducer, milestoneDiffProducer, sepProducer)
	if err != nil {
		_ = snapshotFile.Close()

		return fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[Full], err)
	}

	timeStreamSnapshotData := time.Now()

	// finalize file
	if err := ioutils.CloseFileAndRename(snapshotFile, tempFilePath, filePath); err != nil {
		return err
	}

	if filePath == s.snapshotFullPath {
		// if the old full snapshot file is overwritten
		// we need to remove the old delta snapshot file since it
		// isn't compatible to the full snapshot file anymore.
		if err = os.Remove(s.snapshotDeltaPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting delta snapshot file failed: %s", err)
		}
	}

	timeSetSnapshotInfo := timeStreamSnapshotData
	timeSnapshotMilestoneIndexChanged := timeStreamSnapshotData
	if writeToDatabase {
		// since we write to the database, the targetIndex should exist
		targetMsTimestamp, err := s.storage.MilestoneTimestampByIndex(targetIndex)
		if err != nil {
			return errors.Wrapf(common.ErrCritical, "target milestone (%d) not found", targetIndex)
		}

		if err = s.storage.SetSnapshotIndex(targetIndex, targetMsTimestamp); err != nil {
			s.LogPanic(err)
		}

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

	s.LogInfof("created %s snapshot for target index %d, took %v", snapshotNames[Full], targetIndex, time.Since(ts).Truncate(time.Millisecond))

	return nil
}

// creates a snapshot file by streaming data from the database into a snapshot file.
// the ledger doesn't need to be read locked to stream the changes into the delta snapshot,
// because we don't access the latest ledger state, but only the milestone diffs.
// pruning is also not triggered while the snapshot is ongoing,
// so there is no risk that milestone diffs are pruned in the meantime.
func (s *Manager) createDeltaSnapshotWithoutLocking(ctx context.Context, targetIndex iotago.MilestoneIndex) error {

	s.LogInfof("creating %s snapshot for targetIndex %d", snapshotNames[Delta], targetIndex)
	ts := time.Now()

	s.setIsSnapshotting(true)
	defer s.setIsSnapshotting(false)

	timeStart := time.Now()

	if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		// do not create the snapshot if the node was shut down
		return err
	}

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(common.ErrCritical, common.ErrSnapshotInfoNotFound.Error())
	}

	if err := checkSnapshotLimits(
		snapshotInfo,
		s.syncManager.ConfirmedMilestoneIndex(),
		targetIndex,
		false,
		s.solidEntryPointCheckThresholdPast,
		s.solidEntryPointCheckThresholdFuture,
		true, // if we write the snapshot state to the database, the newly generated snapshot index must be greater than the last snapshot index
	); err != nil {
		return err
	}

	cachedMilestoneTarget := s.storage.CachedMilestoneByIndexOrNil(targetIndex) // milestone +1
	if cachedMilestoneTarget == nil {
		return errors.Wrapf(common.ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedMilestoneTarget.Release(true) // milestone -1

	targetMilestoneTimestamp := cachedMilestoneTarget.Milestone().TimestampUnix()

	timeInit := time.Now()

	// FullSnapshotTargetMilestoneID corresponds to the TargetMilestoneID of the full snapshot.
	// this will return an error if the full snapshot file is not available
	fullHeader, err := s.readSnapshotHeaderFromFullSnapshotFile()
	if err != nil {
		return err
	}

	deltaHeader := &DeltaSnapshotHeader{
		Version:                       SupportedFormatVersion,
		Type:                          Delta,
		TargetMilestoneIndex:          targetIndex,
		TargetMilestoneTimestamp:      targetMilestoneTimestamp,
		FullSnapshotTargetMilestoneID: fullHeader.TargetMilestoneID,
		SEPFileOffset:                 0,
		MilestoneDiffCount:            0,
		SEPCount:                      0,
	}

	_, err = os.Stat(s.snapshotDeltaPath)
	deltaSnapshotFileExists := !os.IsNotExist(err)

	sepProducer := NewSEPsProducer(ctx, s.storage, targetIndex, s.solidEntryPointCheckThresholdPast)

	var snapshotMetrics *Metrics
	var snapshotFile *os.File
	var tempFilePath string

	// a delta snapshot contains the milestone diffs from a full snapshot's target index onwards.
	// if the delta snapshot already exists, we can reuse the existing file and just append to it.
	if deltaSnapshotFileExists {
		var oldDeltaHeader *DeltaSnapshotHeader
		oldDeltaHeader, err = ReadDeltaSnapshotHeaderFromFile(s.snapshotDeltaPath)
		if err != nil {
			return fmt.Errorf("unable to read delta snapshot header: %w", err)
		}

		// we stream the diff from the old delta header target index to the new target index
		milestoneDiffProducer := NewMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxoManager, MsDiffDirectionOnwards, oldDeltaHeader.TargetMilestoneIndex, targetIndex)

		tempFilePath = s.snapshotDeltaPath + "_tmp"
		if err := os.Rename(s.snapshotDeltaPath, tempFilePath); err != nil {
			return fmt.Errorf("unable to rename file: %w", err)
		}

		snapshotFile, err = os.OpenFile(tempFilePath, os.O_RDWR, 0666)
		if err != nil {
			return fmt.Errorf("unable to open existing delta snapshot file: %w", err)
		}

		snapshotMetrics, err = StreamDeltaSnapshotDataToExisting(snapshotFile, deltaHeader, milestoneDiffProducer, sepProducer)

	} else {
		// we stream the diff from the full header target index to the new target index
		milestoneDiffProducer := NewMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxoManager, MsDiffDirectionOnwards, fullHeader.TargetMilestoneIndex, targetIndex)

		snapshotFile, tempFilePath, err = ioutils.CreateTempFile(s.snapshotDeltaPath)
		if err != nil {
			return err
		}

		snapshotMetrics, err = StreamDeltaSnapshotDataTo(snapshotFile, deltaHeader, milestoneDiffProducer, sepProducer)
	}

	if err != nil {
		_ = snapshotFile.Close()

		return fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[Delta], err)
	}

	timeStreamSnapshotData := time.Now()

	// finalize file
	if err := ioutils.CloseFileAndRename(snapshotFile, tempFilePath, s.snapshotDeltaPath); err != nil {
		return err
	}

	timeSetSnapshotInfo := timeStreamSnapshotData
	timeSnapshotMilestoneIndexChanged := timeStreamSnapshotData

	// since we write to the database, the targetIndex should exist
	targetMsTimestamp, err := s.storage.MilestoneTimestampByIndex(targetIndex)
	if err != nil {
		return errors.Wrapf(common.ErrCritical, "target milestone (%d) not found", targetIndex)
	}

	if err = s.storage.SetSnapshotIndex(targetIndex, targetMsTimestamp); err != nil {
		s.LogPanic(err)
	}

	timeSetSnapshotInfo = time.Now()
	s.Events.SnapshotMilestoneIndexChanged.Trigger(targetIndex)
	timeSnapshotMilestoneIndexChanged = time.Now()

	snapshotMetrics.DurationInit = timeInit.Sub(timeStart)
	snapshotMetrics.DurationSetSnapshotInfo = timeSetSnapshotInfo.Sub(timeStreamSnapshotData)
	snapshotMetrics.DurationSnapshotMilestoneIndexChanged = timeSnapshotMilestoneIndexChanged.Sub(timeSetSnapshotInfo)
	snapshotMetrics.DurationTotal = time.Since(timeStart)

	s.Events.SnapshotMetricsUpdated.Trigger(snapshotMetrics)

	s.LogInfof("created %s snapshot for target index %d, took %v", snapshotNames[Delta], targetIndex, time.Since(ts).Truncate(time.Millisecond))

	return nil
}

// creates a full snapshot file by streaming data from the database into a snapshot file.
// this should only be used by MergeSnapshotFiles, otherwise the SEP indexes won't be correct.
func createFullSnapshotFromMergedSnapshotStorageState(dbStorage *storage.Storage, filePath string) (*FullSnapshotHeader, error) {

	snapshotInfo := dbStorage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, errors.Wrap(common.ErrCritical, common.ErrSnapshotInfoNotFound.Error())
	}

	// ledger index corresponds to the target index of the delta snapshot
	ledgerIndex, err := dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, err
	}

	targetIndex := snapshotInfo.EntryPointIndex()

	// if we create a snapshot from the current database state, the solid entry point index needs to match the ledger index.
	// otherwise we need to walk the tangle to calculate the solid entry points and add all milestone diffs until this point.
	if ledgerIndex != targetIndex {
		return nil, errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchTargetIndex, "%d != %d", ledgerIndex, snapshotInfo.EntryPointIndex())
	}

	protoParamsMsOption, err := dbStorage.ProtocolParametersMilestoneOption(ledgerIndex)
	if err != nil {
		return nil, fmt.Errorf("loading protocol parameters milestone option failed: %w", err)
	}

	// TODO: needs to be adapted for when protocol parameters struct changes
	protoParams := &iotago.ProtocolParameters{}
	if _, err := protoParams.Deserialize(protoParamsMsOption.Params, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, fmt.Errorf("failed to deserialize protocol parameters: %w", err)
	}

	cachedMilestoneTarget := dbStorage.CachedMilestoneByIndexOrNil(targetIndex) // milestone +1
	if cachedMilestoneTarget == nil {
		return nil, errors.Wrapf(common.ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedMilestoneTarget.Release(true) // milestone -1

	targetMilestoneTimestamp := cachedMilestoneTarget.Milestone().TimestampUnix()
	targetMilestoneID := cachedMilestoneTarget.Milestone().MilestoneID()

	// read out treasury tx
	unspentTreasuryOutput, err := dbStorage.UTXOManager().UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, fmt.Errorf("unable to get unspent treasury output: %w", err)
	}

	fullHeader := &FullSnapshotHeader{
		Version:                    SupportedFormatVersion,
		Type:                       Full,
		GenesisMilestoneIndex:      snapshotInfo.GenesisMilestoneIndex(),
		TargetMilestoneIndex:       targetIndex,
		TargetMilestoneTimestamp:   targetMilestoneTimestamp,
		TargetMilestoneID:          targetMilestoneID,
		LedgerMilestoneIndex:       ledgerIndex,
		TreasuryOutput:             unspentTreasuryOutput,
		ProtocolParamsMilestoneOpt: protoParamsMsOption,
		OutputCount:                0,
		MilestoneDiffCount:         0,
		SEPCount:                   0,
	}

	// returns a producer which returns all solid entry points in the database.
	sepsCount := 0
	sepProducer := func() SEPProducerFunc {
		prodChan := make(chan interface{})

		go func() {
			dbStorage.ForEachSolidEntryPointWithoutLocking(func(sep *storage.SolidEntryPoint) bool {
				prodChan <- sep.BlockID

				return true
			})
			close(prodChan)
		}()

		binder := producerFromChannels(prodChan, nil)

		return func() (iotago.BlockID, error) {
			obj, err := binder()
			if err != nil {
				return iotago.EmptyBlockID(), err
			}
			if obj == nil {
				return iotago.EmptyBlockID(), ErrNoMoreSEPToProduce
			}
			sepsCount++

			blockID, ok := obj.(iotago.BlockID)
			if !ok {
				return iotago.EmptyBlockID(), fmt.Errorf("expected iotago.BlockID, got %T", obj)
			}

			return blockID, nil
		}
	}()

	cmiUTXOProducer := NewCMIUTXOProducer(dbStorage.UTXOManager())

	// normally we won't have any ms diffs within this merged full snapshot file,
	// but the "AdditionalMilestoneDiffRange" milestone diffs are needed to reconstruct pending protocol parameter updates.
	milestoneDiffProducer := NewMsDiffsProducer(MilestoneRetrieverFromStorage(dbStorage), dbStorage.UTXOManager(), MsDiffDirectionBackwards, targetIndex, targetIndex-AdditionalMilestoneDiffRange)

	snapshotFile, tempFilePath, err := ioutils.CreateTempFile(filePath)
	if err != nil {
		return nil, err
	}

	// stream data into snapshot file
	if _, err := StreamFullSnapshotDataTo(
		snapshotFile,
		fullHeader,
		cmiUTXOProducer,
		milestoneDiffProducer,
		sepProducer); err != nil {
		_ = snapshotFile.Close()

		return nil, fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[Full], err)
	}

	// finalize file
	if err := ioutils.CloseFileAndRename(snapshotFile, tempFilePath, filePath); err != nil {
		return nil, err
	}

	return fullHeader, nil
}

// CreateSnapshotFromStorage creates a snapshot file by streaming data from the database into a snapshot file.
func CreateSnapshotFromStorage(
	ctx context.Context,
	dbStorage *storage.Storage,
	utxoManager *utxo.Manager,
	filePath string,
	targetIndex iotago.MilestoneIndex,
	globalSnapshot bool,
	solidEntryPointCheckThresholdPast iotago.MilestoneIndex,
	solidEntryPointCheckThresholdFuture iotago.MilestoneIndex,
) (*FullSnapshotHeader, error) {

	snapshotInfo := dbStorage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, errors.Wrap(common.ErrCritical, common.ErrSnapshotInfoNotFound.Error())
	}

	// ledger index corresponds to the CMI
	ledgerIndex, err := utxoManager.ReadLedgerIndex()
	if err != nil {
		return nil, err
	}

	// check if the targetIndex is possible
	if err := checkSnapshotLimits(
		snapshotInfo,
		ledgerIndex,
		targetIndex,
		globalSnapshot,
		solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture,
		false); err != nil {
		return nil, err
	}

	// create a temp storage in memory
	//nolint:contextcheck // false positive
	utxoStoreTemp, err := database.StoreWithDefaultSettings("", false, hivedb.EngineMapDB, hivedb.EngineMapDB)
	if err != nil {
		return nil, fmt.Errorf("create temp storage failed: %w", err)
	}

	// copy current ledger state to the temp storage
	if err := kvstore.Copy(dbStorage.UTXOStore(), utxoStoreTemp); err != nil {
		return nil, fmt.Errorf("copy kvstore failed: %w", err)
	}

	// roll back the confirmation in the temporary UTXO manager
	utxoManagerTemp := utxo.New(utxoStoreTemp)

	// we only need to rollback until the resulting ledgerIndex == targetIndex,
	// but everytime we run RollbackConfirmationWithoutLocking we decrease the ledgerIndex by 1.
	// => msIndex > targetIndex is correct in this case.
	for msIndex := ledgerIndex; msIndex > targetIndex; msIndex-- {

		msDiff, err := utxoManagerTemp.MilestoneDiffWithoutLocking(msIndex)
		if err != nil {
			return nil, fmt.Errorf("load milestone diff failed: %w", err)
		}

		var treasuryMutationTuple *utxo.TreasuryMutationTuple
		if msDiff.TreasuryOutput != nil {
			treasuryMutationTuple = &utxo.TreasuryMutationTuple{
				NewOutput:   msDiff.TreasuryOutput,
				SpentOutput: msDiff.SpentTreasuryOutput,
			}
		}

		if err := utxoManagerTemp.RollbackConfirmationWithoutLocking(
			msIndex,
			msDiff.Outputs,
			msDiff.Spents,
			treasuryMutationTuple,
			nil); err != nil {
			return nil, fmt.Errorf("rollback milestone confirmation (%d) failed: %w", msIndex, err)
		}
	}

	// read out treasury tx
	unspentTreasuryOutput, err := utxoManagerTemp.UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, fmt.Errorf("unable to get unspent treasury output: %w", err)
	}

	cachedMilestoneTarget := dbStorage.CachedMilestoneByIndexOrNil(targetIndex) // milestone +1
	if cachedMilestoneTarget == nil {
		return nil, errors.Wrapf(common.ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedMilestoneTarget.Release(true) // milestone -1

	targetMilestoneTimestamp := cachedMilestoneTarget.Milestone().TimestampUnix()
	targetMilestoneID := cachedMilestoneTarget.Milestone().MilestoneID()

	protoParamsMsOption, err := dbStorage.ProtocolParametersMilestoneOption(targetIndex)
	if err != nil {
		return nil, errors.Wrapf(common.ErrCritical, "loading protocol parameters milestone option failed: %s", err.Error())
	}

	fullHeader := &FullSnapshotHeader{
		Version:                    SupportedFormatVersion,
		Type:                       Full,
		GenesisMilestoneIndex:      snapshotInfo.GenesisMilestoneIndex(),
		TargetMilestoneIndex:       targetIndex,
		TargetMilestoneTimestamp:   targetMilestoneTimestamp,
		TargetMilestoneID:          targetMilestoneID,
		LedgerMilestoneIndex:       targetIndex,
		TreasuryOutput:             unspentTreasuryOutput,
		ProtocolParamsMilestoneOpt: protoParamsMsOption,
		OutputCount:                0,
		MilestoneDiffCount:         0,
		SEPCount:                   0,
	}

	var sepProducer SEPProducerFunc
	if globalSnapshot {
		// if we create a global snapshot, we do not need to calculate the SEP.
		// we can simply take the milestone parents of the ledger milestone.
		sepProducer = NewSEPsProducerFromMilestone(cachedMilestoneTarget.Milestone().Milestone())
	} else {
		sepProducer = NewSEPsProducer(ctx, dbStorage, targetIndex, solidEntryPointCheckThresholdPast)
	}

	cmiUTXOProducer := NewCMIUTXOProducer(utxoManagerTemp)

	milestoneDiffProducer := func() (*MilestoneDiff, error) {
		// we won't have any ms diffs within this merged full snapshot file
		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	snapshotFile, tempFilePath, err := ioutils.CreateTempFile(filePath)
	if err != nil {
		return nil, err
	}

	// stream data into snapshot file
	if _, err := StreamFullSnapshotDataTo(
		snapshotFile,
		fullHeader,
		cmiUTXOProducer,
		milestoneDiffProducer,
		sepProducer); err != nil {
		_ = snapshotFile.Close()

		return nil, fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[Full], err)
	}

	// finalize file
	if err := ioutils.CloseFileAndRename(snapshotFile, tempFilePath, filePath); err != nil {
		return nil, err
	}

	return fullHeader, nil
}

// MergeSnapshotsFiles merges the given full and delta snapshots to create an updated full snapshot.
// The result is a full snapshot file containing the ledger outputs corresponding to the
// snapshot index of the specified delta snapshot. The target file does not include any milestone diffs
// and the ledger and snapshot index are equal.
// This function consumes disk space over memory by importing the full snapshot into a temporary database,
// applying the delta diffs onto it and then writing out the merged state.
func MergeSnapshotsFiles(ctx context.Context, fullPath string, deltaPath string, targetFileName string) (*MergeInfo, error) {

	allowedEngines := database.AllowedEnginesStorage

	targetEngine, err := hivedb.EngineAllowed(hivedb.EnginePebble, allowedEngines...)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "snapMerge")
	if err != nil {
		return nil, fmt.Errorf("can't create temp dir: %w", err)
	}

	//nolint:contextcheck // false positive
	tangleStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.TangleDatabaseDirectoryName), true, targetEngine, allowedEngines...)
	if err != nil {
		return nil, fmt.Errorf("%s database initialization failed: %w", coreDatabase.TangleDatabaseDirectoryName, err)
	}

	//nolint:contextcheck // false positive
	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.UTXODatabaseDirectoryName), true, targetEngine, allowedEngines...)
	if err != nil {
		return nil, fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		// clean up temp db
		_ = os.RemoveAll(tempDir)

		return nil, err
	}

	defer func() {
		println("\nshutdown storage ...")
		err := dbStorage.Shutdown()

		// clean up temp db
		_ = os.RemoveAll(tempDir)

		if err != nil {
			panic(err)
		}
	}()

	fullSnapshotHeader, deltaSnapshotHeader, err := LoadSnapshotFilesToStorage(ctx, dbStorage, true, fullPath, deltaPath)
	if err != nil {
		return nil, err
	}

	mergedSnapshotHeader, err := createFullSnapshotFromMergedSnapshotStorageState(dbStorage, targetFileName)
	if err != nil {
		return nil, err
	}

	return &MergeInfo{
		FullSnapshotHeader:   fullSnapshotHeader,
		DeltaSnapshotHeader:  deltaSnapshotHeader,
		MergedSnapshotHeader: mergedSnapshotHeader,
	}, nil
}
