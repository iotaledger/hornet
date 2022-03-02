package snapshot

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
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

// MilestoneRetrieverFunc is a function which returns the milestone for the given index.
type MilestoneRetrieverFunc func(index milestone.Index) (*iotago.Milestone, error)

// MergeInfo holds information about a merge of a full and delta snapshot.
type MergeInfo struct {
	// The header of the full snapshot.
	FullSnapshotHeader *ReadFileHeader
	// The header of the delta snapshot.
	DeltaSnapshotHeader *ReadFileHeader
	// The header of the merged snapshot.
	MergedSnapshotHeader *ReadFileHeader
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

// returns a producer which produces solid entry points.
func newSEPsProducer(ctx context.Context, s *SnapshotManager, targetIndex milestone.Index) SEPProducerFunc {
	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		// calculate solid entry points for the target index
		if err := s.forEachSolidEntryPoint(
			ctx,
			targetIndex,
			func(sep *storage.SolidEntryPoint) bool {
				prodChan <- sep.MessageID
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
		return obj.(*utxo.Output), nil
	}
}

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
func newMsDiffsProducerDeltaFileAndDatabase(snapshotDeltaPath string, dbStorage *storage.Storage, utxoManager *utxo.Manager, ledgerIndex milestone.Index, targetIndex milestone.Index, deSeriParas *iotago.DeSerializationParameters) (MilestoneDiffProducerFunc, error) {
	prevDeltaFileMsDiffsProducer, err := newMsDiffsFromPreviousDeltaSnapshot(snapshotDeltaPath, ledgerIndex, deSeriParas)
	if err != nil {
		return nil, err
	}

	var prevDeltaMsDiffProducerFinished bool
	var prevDeltaUpToIndex = ledgerIndex
	var dbMsDiffProducer MilestoneDiffProducerFunc
	mrf := MilestoneRetrieverFromStorage(dbStorage)
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
func newMsDiffsFromPreviousDeltaSnapshot(snapshotDeltaPath string, originLedgerIndex milestone.Index, deSeriParas *iotago.DeSerializationParameters) (MilestoneDiffProducerFunc, error) {
	existingDeltaFile, err := os.OpenFile(snapshotDeltaPath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("unable to read previous delta snapshot file for milestone diffs: %w", err)
	}

	prodChan := make(chan interface{})
	errChan := make(chan error)

	go func() {
		defer func() { _ = existingDeltaFile.Close() }()

		if err := StreamSnapshotDataFrom(existingDeltaFile,
			deSeriParas,
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

// MilestoneRetrieverFromStorage creates a MilestoneRetrieverFunc which access the storage.
// If it can not retrieve a wanted milestone it panics.
func MilestoneRetrieverFromStorage(dbStorage *storage.Storage) MilestoneRetrieverFunc {
	return func(index milestone.Index) (*iotago.Milestone, error) {
		cachedMsgMilestone := dbStorage.MilestoneCachedMessageOrNil(index) // message +1
		if cachedMsgMilestone == nil {
			return nil, fmt.Errorf("message for milestone with index %d is not stored in the database", index)
		}
		defer cachedMsgMilestone.Release() // message -1
		return cachedMsgMilestone.Message().Milestone(), nil
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
			diff, err := utxoManager.MilestoneDiffWithoutLocking(msIndex)
			if err != nil {
				errChan <- err
				close(prodChan)
				close(errChan)
				return
			}

			ms, err := mrf(msIndex)
			if err != nil {
				errChan <- fmt.Errorf("message for milestone with index %d could not be retrieved: %w", msIndex, err)
				close(prodChan)
				close(errChan)
				return
			}
			if ms == nil {
				errChan <- fmt.Errorf("message for milestone with index %d could not be retrieved", msIndex)
				close(prodChan)
				close(errChan)
				return
			}

			prodChan <- &MilestoneDiff{
				Milestone:           ms,
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
		return obj.(*MilestoneDiff), nil
	}
}

// reads out the index of the milestone which currently represents the ledger state.
func (s *SnapshotManager) readLedgerIndex() (milestone.Index, error) {
	ledgerMilestoneIndex, err := s.utxoManager.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return 0, fmt.Errorf("unable to read current ledger index: %w", err)
	}

	cachedMilestone := s.storage.CachedMilestoneOrNil(ledgerMilestoneIndex) // milestone +1
	if cachedMilestone == nil {
		return 0, errors.Wrapf(ErrCritical, "milestone (%d) not found!", ledgerMilestoneIndex)
	}
	cachedMilestone.Release(true) // milestone -1
	return ledgerMilestoneIndex, nil
}

// reads out the snapshot milestone index from the full snapshot file.
func (s *SnapshotManager) readSnapshotIndexFromFullSnapshotFile(snapshotFullPath ...string) (milestone.Index, error) {
	filePath := s.snapshotFullPath
	if len(snapshotFullPath) > 0 && snapshotFullPath[0] != "" {
		filePath = snapshotFullPath[0]
	}

	fullSnapshotHeader, err := ReadSnapshotHeaderFromFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("unable to read full snapshot header for origin snapshot milestone index: %w", err)
	}

	// note that a full snapshot contains the ledger to the CMI of the node which generated it,
	// however, the state is rolled backed to the snapshot index, therefore, the snapshot index
	// is the actual point from which on the delta snapshot should contain milestone diffs
	return fullSnapshotHeader.SEPMilestoneIndex, nil
}

// returns the timestamp of the target milestone.
func (s *SnapshotManager) readTargetMilestoneTimestamp(targetIndex milestone.Index) (time.Time, error) {
	cachedMilestoneTarget := s.storage.CachedMilestoneOrNil(targetIndex) // milestone +1
	if cachedMilestoneTarget == nil {
		return time.Time{}, errors.Wrapf(ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedMilestoneTarget.Release(true) // milestone -1

	ts := cachedMilestoneTarget.Milestone().Timestamp
	return ts, nil
}

// creates a snapshot file by streaming data from the database into a snapshot file.
func (s *SnapshotManager) createSnapshotWithoutLocking(
	ctx context.Context,
	snapshotType Type,
	targetIndex milestone.Index,
	filePath string,
	writeToDatabase bool,
	snapshotFullPath ...string) error {

	s.LogInfof("creating %s snapshot for targetIndex %d", snapshotNames[snapshotType], targetIndex)
	ts := time.Now()

	s.setIsSnapshotting(true)
	defer s.setIsSnapshotting(false)

	timeStart := time.Now()

	s.utxoManager.ReadLockLedger()
	defer s.utxoManager.ReadUnlockLedger()

	if err := utils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		// do not create the snapshot if the node was shut down
		return err
	}

	timeReadLockLedger := time.Now()

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(ErrCritical, "no snapshot info found")
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

	targetMsTimestamp, err := s.readTargetMilestoneTimestamp(targetIndex)
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
		header.TreasuryOutput, err = s.utxoManager.UnspentTreasuryOutputWithoutLocking()
		if err != nil {
			return err
		}

		// a full snapshot contains the ledger UTXOs as of the CMI
		// and the milestone diffs from the CMI back to the target index (excluding the target index)
		utxoProducer = newCMIUTXOProducer(s.utxoManager)
		milestoneDiffProducer = newMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxoManager, MsDiffDirectionBackwards, header.LedgerMilestoneIndex, targetIndex)

	case Delta:
		// ledger index corresponds to the origin snapshot snapshot ledger.
		// this will return an error if the full snapshot file is not available
		header.LedgerMilestoneIndex, err = s.readSnapshotIndexFromFullSnapshotFile(snapshotFullPath...)
		if err != nil {
			return err
		}

		// a delta snapshot contains the milestone diffs from a full snapshot's snapshot index onwards
		_, err := os.Stat(s.snapshotDeltaPath)
		deltaSnapshotFileExists := !os.IsNotExist(err)

		// if a delta snapshot is created via API, either the internal full snapshot file of the node or a newly created full snapshot file is used ("snapshotFullPath").
		// if the internal full snapshot file is used, the existing delta snapshot file contains the needed data.
		// if a newly created full snapshot file is used, the milestone diffs exist in the database anyway, since the full snapshot limits passed the check (already needed to calculate SEP).
		switch {
		case snapshotInfo.SnapshotIndex == snapshotInfo.PruningIndex && !deltaSnapshotFileExists:
			// when booting up the first time on a full snapshot or in combination with a delta
			// snapshot, this indices will be the same. however, if we have a delta snapshot, we use it
			// since we might not have the actual milestone data.
			fallthrough
		case snapshotInfo.PruningIndex < header.LedgerMilestoneIndex:
			// we have the needed milestone diffs in the database
			milestoneDiffProducer = newMsDiffsProducer(MilestoneRetrieverFromStorage(s.storage), s.utxoManager, MsDiffDirectionOnwards, header.LedgerMilestoneIndex, targetIndex)
		default:
			// as the needed milestone diffs are pruned from the database, we need to use
			// the previous delta snapshot file to extract those in conjunction with what the database has available
			milestoneDiffProducer, err = newMsDiffsProducerDeltaFileAndDatabase(s.snapshotDeltaPath, s.storage, s.utxoManager, header.LedgerMilestoneIndex, targetIndex, s.deSeriParas)
			if err != nil {
				return err
			}
		}
	}

	timeInit := time.Now()

	snapshotFile, tempFilePath, err := utils.CreateTempFile(filePath)
	if err != nil {
		return err
	}

	// stream data into snapshot file
	snapshotMetrics, err := StreamSnapshotDataTo(snapshotFile, uint64(targetMsTimestamp.Unix()), header, newSEPsProducer(ctx, s, targetIndex), utxoProducer, milestoneDiffProducer)
	if err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[snapshotType], err)
	}

	timeStreamSnapshotData := time.Now()

	// finalize file
	if err := utils.CloseFileAndRename(snapshotFile, tempFilePath, filePath); err != nil {
		return err
	}

	if (snapshotType == Full) && (filePath == s.snapshotFullPath) {
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
		targetMsTimestamp, err := s.readTargetMilestoneTimestamp(targetIndex)
		if err != nil {
			return err
		}

		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = targetMsTimestamp
		if err = s.storage.SetSnapshotInfo(snapshotInfo); err != nil {
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

	s.LogInfof("created %s snapshot for target index %d, took %v", snapshotNames[snapshotType], targetIndex, time.Since(ts).Truncate(time.Millisecond))
	return nil
}

// creates a snapshot file by streaming data from the database into a snapshot file.
func createSnapshotFromCurrentStorageState(dbStorage *storage.Storage, filePath string) (*ReadFileHeader, error) {

	snapshotInfo := dbStorage.SnapshotInfo()
	if snapshotInfo == nil {
		return nil, errors.Wrap(ErrCritical, "no snapshot info found")
	}

	ledgerIndex, err := dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, err
	}

	// if we create a snapshot from the current database state, the solid entry point index needs to match the ledger index.
	// otherwise we need to walk the tangle to calculate the solid entry points and add all milestone diffs until this point.
	if ledgerIndex != snapshotInfo.EntryPointIndex {
		return nil, errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchSEPIndex, "%d != %d", ledgerIndex, snapshotInfo.EntryPointIndex)
	}

	// read out treasury tx
	unspentTreasuryOutput, err := dbStorage.UTXOManager().UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return nil, fmt.Errorf("unable to get unspent treasury output: %w", err)
	}

	snapshotFileHeader := &FileHeader{
		Version:              SupportedFormatVersion,
		Type:                 Full,
		NetworkID:            snapshotInfo.NetworkID,
		SEPMilestoneIndex:    ledgerIndex,
		LedgerMilestoneIndex: ledgerIndex,
		TreasuryOutput:       unspentTreasuryOutput,
	}

	// returns a producer which returns all solid entry points in the database.
	sepsCount := 0
	sepProducer := func() SEPProducerFunc {
		prodChan := make(chan interface{})

		go func() {
			dbStorage.ForEachSolidEntryPointWithoutLocking(func(sep *storage.SolidEntryPoint) bool {
				prodChan <- sep.MessageID
				return true
			})
			close(prodChan)
		}()

		binder := producerFromChannels(prodChan, nil)
		return func() (hornet.MessageID, error) {
			obj, err := binder()
			if obj == nil || err != nil {
				return nil, err
			}
			sepsCount++
			return obj.(hornet.MessageID), nil
		}
	}()

	// create a prepped output producer which counts how many went through
	unspentOutputsCount := 0
	cmiUTXOProducer := newCMIUTXOProducer(dbStorage.UTXOManager())
	countingOutputProducer := func() (*utxo.Output, error) {
		output, err := cmiUTXOProducer()
		if output != nil {
			unspentOutputsCount++
		}
		return output, err
	}

	milestoneDiffProducer := func() (*MilestoneDiff, error) {
		// we won't have any ms diffs within this merged full snapshot file
		return nil, nil
	}

	snapshotFile, tempFilePath, err := utils.CreateTempFile(filePath)
	if err != nil {
		return nil, err
	}

	// stream data into snapshot file
	if _, err := StreamSnapshotDataTo(
		snapshotFile,
		uint64(snapshotInfo.Timestamp.Unix()),
		snapshotFileHeader,
		sepProducer,
		countingOutputProducer,
		milestoneDiffProducer); err != nil {
		_ = snapshotFile.Close()
		return nil, fmt.Errorf("couldn't generate %s snapshot file: %w", snapshotNames[Full], err)
	}

	// finalize file
	if err := utils.CloseFileAndRename(snapshotFile, tempFilePath, filePath); err != nil {
		return nil, err
	}

	return &ReadFileHeader{
		FileHeader:         *snapshotFileHeader,
		Timestamp:          uint64(snapshotInfo.Timestamp.Unix()),
		SEPCount:           uint64(sepsCount),
		OutputCount:        uint64(unspentOutputsCount),
		MilestoneDiffCount: 0,
	}, nil
}

// MergeSnapshotsFiles merges the given full and delta snapshots to create an updated full snapshot.
// The result is a full snapshot file containing the ledger outputs corresponding to the
// snapshot index of the specified delta snapshot. The target file does not include any milestone diffs
// and the ledger and snapshot index are equal.
// This function consumes disk space over memory by importing the full snapshot into a temporary database,
// applying the delta diffs onto it and then writing out the merged state.
func MergeSnapshotsFiles(fullPath string, deltaPath string, targetFileName string, deSeriParas *iotago.DeSerializationParameters) (*MergeInfo, error) {

	targetEngine, err := database.DatabaseEngine(string(database.EnginePebble))
	if err != nil {
		return nil, err
	}

	tempDir, err := ioutil.TempDir("", "snapMerge")
	if err != nil {
		return nil, fmt.Errorf("can't create temp dir: %w", err)
	}

	tangleStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.TangleDatabaseDirectoryName), true, targetEngine)
	if err != nil {
		return nil, fmt.Errorf("%s database initialization failed: %w", coreDatabase.TangleDatabaseDirectoryName, err)
	}

	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.UTXODatabaseDirectoryName), true, targetEngine)
	if err != nil {
		return nil, fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	// clean up temp db
	defer func() {
		tangleStore.Shutdown()
		_ = tangleStore.Close()

		utxoStore.Shutdown()
		_ = utxoStore.Close()

		_ = os.RemoveAll(tempDir)
	}()

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		return nil, err
	}

	fullSnapshotHeader, deltaSnapshotHeader, err := LoadSnapshotFilesToStorage(context.Background(), dbStorage, deSeriParas, fullPath, deltaPath)
	if err != nil {
		return nil, err
	}

	mergedSnapshotHeader, err := createSnapshotFromCurrentStorageState(dbStorage, targetFileName)
	if err != nil {
		return nil, err
	}

	return &MergeInfo{
		FullSnapshotHeader:   fullSnapshotHeader,
		DeltaSnapshotHeader:  deltaSnapshotHeader,
		MergedSnapshotHeader: mergedSnapshotHeader,
	}, nil
}
