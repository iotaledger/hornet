package snapshot

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

const (
	SolidEntryPointCheckThresholdPast   = 50
	SolidEntryPointCheckThresholdFuture = 50
)

var (
	// Returned when a critical error stops the execution of a task.
	ErrCritical = errors.New("critical error")
	// Returned when an unsupported local snapshot file version is read.
	ErrUnsupportedLSFileVersion = errors.New("unsupported local snapshot file version")
	// Returned when a child message wasn't found.
	ErrChildMsgNotFound = errors.New("child message not found")
	// Returned when the milestone diff that should be applied is not the current or next milestone.
	ErrWrongMilestoneDiffIndex = errors.New("wrong milestone diff index")
	// Returned when the final milestone after loading the snapshot is not equal to the solid entry point index.
	ErrFinalLedgerIndexDoesNotMatchSEPIndex = errors.New("final ledger index does not match solid entry point index")
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

// getMilestoneParents traverses a milestone and collects all messages that were confirmed by that milestone or newer.
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

func forEachSolidEntryPoint(targetIndex milestone.Index, abortSignal <-chan struct{}, solidEntryPointConsumer func(solidEntryPointMessageID hornet.Hash, index milestone.Index) bool) error {

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

		cachedMilestone := tangle.GetCachedMilestoneOrNil(milestoneIndex) // milestone +1
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
				cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(parentMessageID)
				if cachedMsgMeta == nil {
					return errors.Wrapf(ErrCritical, "metadata (%v) not found!", parentMessageID.Hex())
				}

				confirmed, at := cachedMsgMeta.GetMetadata().GetConfirmed()
				if !confirmed {
					cachedMsgMeta.Release(true)
					return errors.Wrapf(ErrCritical, "solid entry point (%v) not confirmed!", parentMessageID.Hex())
				}
				cachedMsgMeta.Release(true)

				if _, exists := solidEntryPoints[string(parentMessageID)]; !exists {
					solidEntryPoints[string(parentMessageID)] = at
					if !solidEntryPointConsumer(parentMessageID, at) {
						return ErrSnapshotCreationWasAborted
					}
				}
			}
		}
	}

	return nil
}

func checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *tangle.SnapshotInfo, checkSnapshotIndex bool) error {

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

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

	header := &FileHeader{
		Version:           SupportedFormatVersion,
		Type:              Full,
		SEPMilestoneIndex: milestone.Index(targetIndex),
	}
	copy(header.SEPMilestoneHash[:], cachedTargetMilestone.GetMilestone().MessageID)

	// build temp file path
	filePathTmp := filePath + "_tmp"
	_ = os.Remove(filePathTmp)

	// create temp file
	lsFile, err := os.OpenFile(filePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create tmp local snapshot file: %w", err)
	}

	utxo.ReadLockLedger()
	defer utxo.ReadUnlockLedger()

	//
	// solid entry points
	//
	solidEntryPointProducerChan := make(chan *[SolidEntryPointHashLength]byte)
	solidEntryPointProducerErrorChan := make(chan error)

	solidEntryPointProducerFunc := func() (*[SolidEntryPointHashLength]byte, error) {
		select {
		case err, ok := <-solidEntryPointProducerErrorChan:
			if !ok {
				return nil, nil
			}
			return nil, err
		case sep, ok := <-solidEntryPointProducerChan:
			if !ok {
				return nil, nil
			}
			return sep, nil
		}
	}

	go func() {
		// calculate solid entry points for the target index
		if err := forEachSolidEntryPoint(targetIndex, abortSignal, func(solidEntryPointMessageID hornet.Hash, index milestone.Index) bool {

			var solidEntryPoint [SolidEntryPointHashLength]byte
			copy(solidEntryPoint[:], solidEntryPointMessageID)

			solidEntryPointProducerChan <- &solidEntryPoint
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
		if err := utxo.ForEachUnspentOutputWithoutLocking(func(output *utxo.Output) bool {
			outputProducerChan <- &Output{OutputID: output.OutputID, Address: &output.Address, Amount: output.Amount}
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
		for msIndex, _ := utxo.ReadLedgerIndexWithoutLocking(); msIndex >= targetIndex; msIndex-- {
			newOutputs, newSpents, err := utxo.GetMilestoneDiffsWithoutLocking(msIndex)
			if err != nil {
				milestoneDiffProducerErrorChan <- err
				close(milestoneDiffProducerChan)
				close(milestoneDiffProducerErrorChan)
				return
			}

			var createdOutputs []*Output
			var consumedOutputs []*Spent

			for _, createdOutput := range newOutputs {
				createdOutputs = append(createdOutputs, &Output{OutputID: createdOutput.OutputID, Address: &createdOutput.Address, Amount: createdOutput.Amount})
			}

			for _, consumedOutput := range newSpents {
				consumedOutputs = append(consumedOutputs, &Spent{Output: Output{OutputID: consumedOutput.OutputID, Address: &consumedOutput.Address, Amount: consumedOutput.Output.Amount}, TargetTransactionID: consumedOutput.TargetTransactionID})
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

	// re-read the local snapshot file to compute its hash.
	// note that this step can only be done after the local snapshot file
	// has been written since the ls file generation uses seeking.
	lsFileHash, err := localSnapshotFileHash(filePath)
	if err != nil {
		return err
	}

	log.Infof("created local snapshot for target index %d (sha256: %x), took %v", targetIndex, lsFileHash, time.Since(ts))
	return nil
}

// computes the sha256b hash of the given local snapshot file.
func localSnapshotFileHash(filePath string) ([]byte, error) {
	writtenLsFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open local snapshot file for hash computation: %w", err)
	}
	defer writtenLsFile.Close()

	sha256Hash := sha256.New()
	if _, err := io.Copy(sha256Hash, writtenLsFile); err != nil {
		return nil, fmt.Errorf("unable to copy local snapshot file content for hash computation: %w", err)
	}
	return sha256Hash.Sum(nil), nil
}

func LoadFullSnapshotFromFile(filePath string) error {
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
			return errors.Wrapf(ErrUnsupportedLSFileVersion, "local snapshot file version is %d but this HORNET version only supports %v", header.Version, SupportedFormatVersion)
		}
		lsHeader = header
		log.Infof("solid entry points: %d, outputs: %d, ms diffs: %d", header.SEPCount, header.OutputCount, header.MilestoneDiffCount)

		if err := utxo.StoreLedgerIndex(lsHeader.LedgerMilestoneIndex); err != nil {
			return err
		}

		return nil
	}

	// note that we only get the hash of the SEP message instead
	// of also its associated oldest cone root index, since the index
	// of the local snapshot milestone will be below max depth anyway.
	// this information was included in pre Chrysalis Phase 2 local snapshots
	// but has been deemed unnecessary for the reason mentioned above.
	sepConsumer := func(sepMsgHashBytes [32]byte) error {
		tangle.SolidEntryPointsAdd(sepMsgHashBytes[:], lsHeader.SEPMilestoneIndex)
		return nil
	}

	outputConsumer := func(unspentOutput *Output) error {
		switch addr := unspentOutput.Address.(type) {
		case *iotago.WOTSAddress:
			return iotago.ErrWOTSNotImplemented
		case *iotago.Ed25519Address:
			return utxo.AddUnspentOutput(&utxo.Output{OutputID: unspentOutput.OutputID, Address: *addr, Amount: unspentOutput.Amount})
		default:
			return iotago.ErrUnknownAddrType
		}
	}

	msDiffConsumer := func(msDiff *MilestoneDiff) error {
		var newOutputs []*utxo.Output
		var newSpents []*utxo.Spent

		for _, createdOutput := range msDiff.Created {
			switch addr := createdOutput.Address.(type) {
			case *iotago.WOTSAddress:
				return iotago.ErrWOTSNotImplemented
			case *iotago.Ed25519Address:
				newOutputs = append(newOutputs, &utxo.Output{OutputID: createdOutput.OutputID, Address: *addr, Amount: createdOutput.Amount})
			default:
				return iotago.ErrUnknownAddrType
			}
		}

		for _, consumedOutput := range msDiff.Consumed {
			switch addr := consumedOutput.Address.(type) {
			case *iotago.WOTSAddress:
				return iotago.ErrWOTSNotImplemented
			case *iotago.Ed25519Address:
				newSpents = append(newSpents, utxo.NewSpent(&utxo.Output{OutputID: consumedOutput.OutputID, Address: *addr, Amount: consumedOutput.Amount}, consumedOutput.TargetTransactionID, msDiff.MilestoneIndex))
			default:
				return iotago.ErrUnknownAddrType
			}
		}

		ledgerIndex, err := utxo.ReadLedgerIndex()
		if err != nil {
			return err
		}

		if ledgerIndex == msDiff.MilestoneIndex {
			return utxo.RollbackConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		}

		if ledgerIndex == msDiff.MilestoneIndex+1 {
			return utxo.ApplyConfirmation(msDiff.MilestoneIndex, newOutputs, newSpents)
		}

		return ErrWrongMilestoneDiffIndex
	}

	tangle.WriteLockSolidEntryPoints()
	tangle.ResetSolidEntryPoints()
	defer tangle.WriteUnlockSolidEntryPoints()
	defer tangle.StoreSolidEntryPoints()

	if err := StreamLocalSnapshotDataFrom(lsFile, headerConsumer, sepConsumer, outputConsumer, msDiffConsumer); err != nil {
		return fmt.Errorf("unable to import local snapshot file: %w", err)
	}

	log.Infof("imported local snapshot file, took %v", time.Since(s))

	cooPublicKey, err := utils.ParseEd25519PublicKeyFromString(config.NodeConfig.GetString(config.CfgCoordinatorPublicKey))
	if err != nil {
		return err
	}

	utxo.CheckLedgerState()

	ledgerIndex, err := utxo.ReadLedgerIndex()
	if err != nil {
		return err
	}

	if ledgerIndex != lsHeader.SEPMilestoneIndex {
		return ErrFinalLedgerIndexDoesNotMatchSEPIndex
	}

	tangle.SetSnapshotMilestone(cooPublicKey, lsHeader.SEPMilestoneHash[:], lsHeader.SEPMilestoneIndex, lsHeader.SEPMilestoneIndex, lsHeader.SEPMilestoneIndex, time.Now())
	tangle.SetSolidMilestoneIndex(lsHeader.SEPMilestoneIndex, false)

	return nil
}
