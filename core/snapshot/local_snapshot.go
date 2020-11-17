package snapshot

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
)

const (
	SolidEntryPointCheckThresholdPast   = 50
	SolidEntryPointCheckThresholdFuture = 50
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
)

type solidEntryPoint struct {
	messageID *hornet.MessageID
	index     milestone.Index
}

// isSolidEntryPoint checks whether any direct child of the given message was referenced by a milestone which is above the target milestone.
func isSolidEntryPoint(messageID *hornet.MessageID, targetIndex milestone.Index) bool {

	for _, childMessageID := range deps.Storage.GetChildrenMessageIDs(messageID) {
		cachedMsgMeta := deps.Storage.GetCachedMessageMetadataOrNil(childMessageID) // meta +1
		if cachedMsgMeta == nil {
			// Ignore this message since it doesn't exist anymore
			log.Warnf("%s, msg ID: %v, child msg ID: %v", ErrChildMsgNotFound, messageID.Hex(), childMessageID.Hex())
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
func getMilestoneParentMessageIDs(milestoneIndex milestone.Index, milestoneMessageID *hornet.MessageID, abortSignal <-chan struct{}) (hornet.MessageIDs, error) {

	var parentMessageIDs hornet.MessageIDs

	ts := time.Now()

	if err := dag.TraverseParents(deps.Storage, milestoneMessageID,
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

	log.Debugf("milestone walked (%d): parents: %v, collect: %v", milestoneIndex, len(parentMessageIDs), time.Since(ts))
	return parentMessageIDs, nil
}

func shouldTakeSnapshot(solidMilestoneIndex milestone.Index) bool {

	snapshotInfo := deps.Storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	var snapshotInterval milestone.Index
	if deps.Storage.IsNodeSynced() {
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

func forEachSolidEntryPoint(targetIndex milestone.Index, abortSignal <-chan struct{}, solidEntryPointConsumer func(sep *solidEntryPoint) bool) error {

	solidEntryPoints := make(map[string]milestone.Index)

	// HINT: Check if "old solid entry points are still valid" is skipped in HORNET,
	//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
	//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old msg as valid tips, which is not the case anymore.

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - SolidEntryPointCheckThresholdPast; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		cachedMilestone := deps.Storage.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			return errors.Wrapf(ErrCritical, "milestone (%d) not found!", milestoneIndex)
		}

		// Get all parents of that milestone
		milestoneMessageID := cachedMilestone.GetMilestone().MessageID
		cachedMilestone.Release(true) // message -1

		parentMessageIDs, err := getMilestoneParentMessageIDs(milestoneIndex, milestoneMessageID, abortSignal)
		if err != nil {
			return err
		}

		for _, parentMessageID := range parentMessageIDs {
			select {
			case <-abortSignal:
				return ErrSnapshotCreationWasAborted
			default:
			}

			if isEntryPoint := isSolidEntryPoint(parentMessageID, targetIndex); isEntryPoint {
				cachedMsgMeta := deps.Storage.GetCachedMessageMetadataOrNil(parentMessageID)
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

func checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *storage.SnapshotInfo, checkSnapshotIndex bool) error {

	solidMilestoneIndex := deps.Storage.GetSolidMilestoneIndex()

	if solidMilestoneIndex < SolidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum solid index: %d, actual solid index: %d", SolidEntryPointCheckThresholdFuture+1, solidMilestoneIndex)
	}

	minimumIndex := milestone.Index(SolidEntryPointCheckThresholdPast + 1)
	maximumIndex := solidMilestoneIndex - SolidEntryPointCheckThresholdFuture

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

func setIsSnapshotting(value bool) {
	statusLock.Lock()
	isSnapshotting = value
	statusLock.Unlock()
}

func CreateLocalSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()
	return createFullLocalSnapshotWithoutLocking(targetIndex, filePath, writeToDatabase, abortSignal)
}

func createFullLocalSnapshotWithoutLocking(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	log.Infof("creating local snapshot for targetIndex %d", targetIndex)
	ts := time.Now()

	snapshotInfo := deps.Storage.GetSnapshotInfo()
	if snapshotInfo == nil {
		return errors.Wrap(ErrCritical, "no snapshot info found")
	}

	if err := checkSnapshotLimits(targetIndex, snapshotInfo, writeToDatabase); err != nil {
		return err
	}

	setIsSnapshotting(true)
	defer setIsSnapshotting(false)

	cachedTargetMilestone := deps.Storage.GetCachedMilestoneOrNil(targetIndex) // milestone +1
	if cachedTargetMilestone == nil {
		return errors.Wrapf(ErrCritical, "target milestone (%d) not found", targetIndex)
	}
	defer cachedTargetMilestone.Release(true) // milestone -1

	header := &FileHeader{
		Version:           SupportedFormatVersion,
		Type:              Full,
		NetworkID:         snapshotInfo.NetworkID,
		SEPMilestoneIndex: milestone.Index(targetIndex),
		SEPMilestoneID:    cachedTargetMilestone.GetMilestone().MilestoneID,
	}

	// build temp file path
	filePathTmp := filePath + "_tmp"
	_ = os.Remove(filePathTmp)

	// create temp file
	lsFile, err := os.OpenFile(filePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create tmp local snapshot file: %w", err)
	}

	deps.UTXO.ReadLockLedger()
	defer deps.UTXO.ReadUnlockLedger()

	ledgerMilestoneIndex, err := deps.UTXO.ReadLedgerIndexWithoutLocking()
	if err != nil {
		return fmt.Errorf("unable to read current ledger index: %w", err)
	}

	cachedMilestone := deps.Storage.GetCachedMilestoneOrNil(ledgerMilestoneIndex)
	if cachedMilestone == nil {
		return errors.Wrapf(ErrCritical, "milestone (%d) not found!", ledgerMilestoneIndex)
	}

	header.LedgerMilestoneIndex = ledgerMilestoneIndex
	header.LedgerMilestoneID = cachedMilestone.GetMilestone().MilestoneID

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
		if err := forEachSolidEntryPoint(targetIndex, abortSignal, func(sep *solidEntryPoint) bool {
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
		if err := deps.UTXO.ForEachUnspentOutputWithoutLocking(func(output *utxo.Output) bool {
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
			newOutputs, newSpents, err := deps.UTXO.GetMilestoneDiffsWithoutLocking(msIndex)
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
			err = deps.Storage.StoreSnapshotBalancesInDatabase(newBalances, targetIndex)
			if err != nil {
				return errors.Wrap(ErrCritical, err.Error())
			}
		*/

		snapshotInfo.MilestoneID = cachedTargetMilestone.GetMilestone().MilestoneID
		snapshotInfo.SnapshotIndex = targetIndex
		snapshotInfo.Timestamp = cachedTargetMilestone.GetMilestone().Timestamp
		deps.Storage.SetSnapshotInfo(snapshotInfo)

		deps.Tangle.Events.SnapshotMilestoneIndexChanged.Trigger(targetIndex)
	}

	log.Infof("created local snapshot for target index %d, took %v", targetIndex, time.Since(ts))
	return nil
}

func LoadFullSnapshotFromFile(networkID uint64, filePath string) error {
	log.Info("importing full snapshot file...")
	s := time.Now()

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
		log.Infof("solid entry points: %d, outputs: %d, ms diffs: %d", header.SEPCount, header.OutputCount, header.MilestoneDiffCount)

		if err := deps.UTXO.StoreLedgerIndex(lsHeader.LedgerMilestoneIndex); err != nil {
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
		deps.Storage.SolidEntryPointsAdd(solidEntryPointMessageID, lsHeader.SEPMilestoneIndex)
		return nil
	}

	outputConsumer := func(output *Output) error {
		switch addr := output.Address.(type) {
		case *iotago.WOTSAddress:
			return iotago.ErrWOTSNotImplemented
		case *iotago.Ed25519Address:

			outputID := iotago.UTXOInputID(output.OutputID)
			messageID := hornet.MessageID(output.MessageID)

			return deps.UTXO.AddUnspentOutput(utxo.GetOutput(&outputID, &messageID, addr, output.Amount))
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

		ledgerIndex, err := deps.UTXO.ReadLedgerIndex()
		if err != nil {
			return err
		}

		if ledgerIndex == msDiff.MilestoneIndex {
			return deps.UTXO.RollbackConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		}

		if ledgerIndex == msDiff.MilestoneIndex+1 {
			return deps.UTXO.ApplyConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		}

		return ErrWrongMilestoneDiffIndex
	}

	deps.Storage.WriteLockSolidEntryPoints()
	deps.Storage.ResetSolidEntryPoints()
	defer deps.Storage.WriteUnlockSolidEntryPoints()
	defer deps.Storage.StoreSolidEntryPoints()

	if err := StreamLocalSnapshotDataFrom(lsFile, headerConsumer, sepConsumer, outputConsumer, msDiffConsumer); err != nil {
		return fmt.Errorf("unable to import local snapshot file: %w", err)
	}

	log.Infof("imported local snapshot file, took %v", time.Since(s))

	if err := deps.UTXO.CheckLedgerState(); err != nil {
		return err
	}

	ledgerIndex, err := deps.UTXO.ReadLedgerIndex()
	if err != nil {
		return err
	}

	if ledgerIndex != lsHeader.SEPMilestoneIndex {
		return errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchSEPIndex, "%d != %d", ledgerIndex, lsHeader.SEPMilestoneIndex)
	}

	deps.Storage.SetSnapshotMilestone(lsHeader.NetworkID, lsHeader.SEPMilestoneID, lsHeader.SEPMilestoneIndex, lsHeader.SEPMilestoneIndex, lsHeader.SEPMilestoneIndex, time.Now())
	deps.Storage.SetSolidMilestoneIndex(lsHeader.SEPMilestoneIndex, false)

	return nil
}
