package snapshot

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

// returns a file header consumer, which stores the ledger milestone index up on execution in the database.
// the given targetHeader is populated with the value of the read file header.
func newFileHeaderConsumer(targetHeader *ReadFileHeader, utxoManager *utxo.Manager, wantedType Type, wantedNetworkID ...uint64) HeaderConsumerFunc {
	return func(header *ReadFileHeader) error {
		if header.Version != SupportedFormatVersion {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file version is %d but this HORNET version only supports %v", header.Version, SupportedFormatVersion)
		}

		if header.Type != wantedType {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file is of type %s but expected was %s", snapshotNames[header.Type], snapshotNames[wantedType])
		}

		if len(wantedNetworkID) > 0 {
			if header.NetworkID != wantedNetworkID[0] {
				return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file network ID is %d but this HORNET is meant for %d", header.NetworkID, wantedNetworkID[0])
			}
		}

		*targetHeader = *header

		if err := utxoManager.StoreLedgerIndex(header.LedgerMilestoneIndex); err != nil {
			return err
		}

		return nil
	}
}

// returns a solid entry point consumer which stores them into the database.
// the SEPs are stored with the corresponding SEP milestone index from the snapshot.
func newSEPsConsumer(dbStorage *storage.Storage, header *ReadFileHeader) SEPConsumerFunc {
	// note that we only get the hash of the SEP message instead
	// of also its associated oldest cone root index, since the index
	// of the snapshot milestone will be below max depth anyway.
	// this information was included in pre Chrysalis Phase 2 snapshots
	// but has been deemed unnecessary for the reason mentioned above.
	return func(solidEntryPointMessageID hornet.BlockID) error {
		dbStorage.SolidEntryPointsAddWithoutLocking(solidEntryPointMessageID, header.SEPMilestoneIndex)
		return nil
	}
}

// returns an output consumer storing them into the database.
func newOutputConsumer(utxoManager *utxo.Manager) OutputConsumerFunc {
	return func(output *utxo.Output) error {
		return utxoManager.AddUnspentOutput(output)
	}
}

// returns a treasury output consumer which overrides an existing unspent treasury output with the new one.
func newUnspentTreasuryOutputConsumer(utxoManager *utxo.Manager) UnspentTreasuryOutputConsumerFunc {
	// leave like this for now in case we need to do more in the future
	return utxoManager.StoreUnspentTreasuryOutput
}

// creates a milestone diff consumer storing them into the database.
// if the ledger index within the database equals the produced milestone diff's index,
// then its changes are roll-backed, otherwise, if the index is higher than the ledger index,
// its mutations are applied on top of the latest state.
// the caller needs to make sure to set the ledger index accordingly beforehand.
func newMsDiffConsumer(utxoManager *utxo.Manager) MilestoneDiffConsumerFunc {
	return func(msDiff *MilestoneDiff) error {
		msIndex := milestone.Index(msDiff.Milestone.Index)
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
				Receipt:        msDiff.Milestone.Opts.MustSet().Receipt(),
				MilestoneIndex: msIndex,
			}
		}

		switch {
		case ledgerIndex == msIndex:
			return utxoManager.RollbackConfirmation(msIndex, msDiff.Created, msDiff.Consumed, treasuryMut, rt)
		case ledgerIndex+1 == msIndex:
			return utxoManager.ApplyConfirmation(msIndex, msDiff.Created, msDiff.Consumed, treasuryMut, rt)
		default:
			return ErrWrongMilestoneDiffIndex
		}
	}
}

// loadSnapshotFileToStorage loads a snapshot file from the given file path into the storage.
func loadSnapshotFileToStorage(
	shutdownCtx context.Context,
	dbStorage *storage.Storage,
	snapshotType Type,
	filePath string,
	protoParas *iotago.ProtocolParameters) (header *ReadFileHeader, err error) {

	dbStorage.WriteLockSolidEntryPoints()
	dbStorage.ResetSolidEntryPointsWithoutLocking()
	defer func() {
		if errStore := dbStorage.StoreSolidEntryPointsWithoutLocking(); err == nil && errStore != nil {
			err = errStore
		}
		dbStorage.WriteUnlockSolidEntryPoints()
	}()

	var lsFile *os.File
	lsFile, err = os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open %s snapshot file for import: %w", snapshotNames[snapshotType], err)
	}
	defer func() { _ = lsFile.Close() }()

	header = &ReadFileHeader{}
	headerConsumer := newFileHeaderConsumer(header, dbStorage.UTXOManager(), snapshotType, protoParas.NetworkID())
	sepConsumer := newSEPsConsumer(dbStorage, header)
	var outputConsumer OutputConsumerFunc
	var treasuryOutputConsumer UnspentTreasuryOutputConsumerFunc
	if snapshotType == Full {
		// not needed if Delta snapshot is applied
		outputConsumer = newOutputConsumer(dbStorage.UTXOManager())
		treasuryOutputConsumer = newUnspentTreasuryOutputConsumer(dbStorage.UTXOManager())
	}
	msDiffConsumer := newMsDiffConsumer(dbStorage.UTXOManager())

	if err = StreamSnapshotDataFrom(lsFile, protoParas, headerConsumer, sepConsumer, outputConsumer, treasuryOutputConsumer, msDiffConsumer); err != nil {
		return nil, fmt.Errorf("unable to import %s snapshot file: %w", snapshotNames[snapshotType], err)
	}

	if err = dbStorage.UTXOManager().CheckLedgerState(protoParas); err != nil {
		return nil, err
	}

	var ledgerIndex milestone.Index
	ledgerIndex, err = dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, err
	}

	if ledgerIndex != header.SEPMilestoneIndex {
		return nil, errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchSEPIndex, "%d != %d", ledgerIndex, header.SEPMilestoneIndex)
	}

	if err = dbStorage.SetSnapshotMilestone(header.NetworkID, header.SEPMilestoneIndex, header.SEPMilestoneIndex, header.SEPMilestoneIndex, time.Unix(int64(header.Timestamp), 0)); err != nil {
		return nil, fmt.Errorf("SetSnapshotMilestone failed: %w", err)
	}

	return header, nil
}

// LoadSnapshotFilesToStorage loads the snapshot files from the given file paths into the storage.
func LoadSnapshotFilesToStorage(ctx context.Context, dbStorage *storage.Storage, protoParas *iotago.ProtocolParameters, fullPath string, deltaPath ...string) (*ReadFileHeader, *ReadFileHeader, error) {

	if len(deltaPath) > 0 && deltaPath[0] != "" {

		// check that the delta snapshot file's ledger index equals the snapshot index of the full one
		fullHeader, err := ReadSnapshotHeaderFromFile(fullPath)
		if err != nil {
			return nil, nil, err
		}

		deltaHeader, err := ReadSnapshotHeaderFromFile(deltaPath[0])
		if err != nil {
			return nil, nil, err
		}

		if deltaHeader.LedgerMilestoneIndex != fullHeader.SEPMilestoneIndex {
			return nil, nil, fmt.Errorf("%w: delta snapshot's ledger index %d does not correspond to full snapshot's SEPs index %d",
				ErrSnapshotsNotMergeable, deltaHeader.LedgerMilestoneIndex, fullHeader.SEPMilestoneIndex)
		}
	}

	var fullSnapshotHeader, deltaSnapshotHeader *ReadFileHeader
	fullSnapshotHeader, err := loadSnapshotFileToStorage(ctx, dbStorage, Full, fullPath, protoParas)
	if err != nil {
		return nil, nil, err
	}

	if len(deltaPath) > 0 && deltaPath[0] != "" {
		deltaSnapshotHeader, err = loadSnapshotFileToStorage(ctx, dbStorage, Delta, deltaPath[0], protoParas)
		if err != nil {
			return nil, nil, err
		}
	}

	return fullSnapshotHeader, deltaSnapshotHeader, nil
}
