package snapshot

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

// returns an in-memory copy of the ProtocolStorage of the dbStorage.
func newProtocolStorageGetterFunc(dbStorage *storage.Storage) ProtocolStorageGetterFunc {
	return func() (*storage.ProtocolStorage, error) {
		// initialize a temporary protocol storage in memory
		protocolStorage := storage.NewProtocolStorage(mapdb.NewMapDB())

		// copy all existing protocol parameters milestone options to the new storage.
		var innerErr error
		if err := dbStorage.ForEachProtocolParameterMilestoneOption(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
			if err := protocolStorage.StoreProtocolParametersMilestoneOption(protoParamsMsOption); err != nil {
				innerErr = err

				return false
			}

			return true
		}); err != nil {
			return nil, fmt.Errorf("failed to iterate over protocol parameters milestone options: %w", err)
		}
		if innerErr != nil {
			return nil, innerErr
		}

		return protocolStorage, nil
	}
}

// returns a file header consumer, which stores the ledger milestone index up on execution in the database.
// the given targetHeader is populated with the value of the read file header.
func newFullHeaderConsumer(targetFullHeader *FullSnapshotHeader, utxoManager *utxo.Manager, targetNetworkID ...uint64) FullHeaderConsumerFunc {
	return func(header *FullSnapshotHeader) error {
		if header.Version != SupportedFormatVersion {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file version is %d but this HORNET version only supports %v", header.Version, SupportedFormatVersion)
		}

		if header.Type != Full {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file is of type %s but expected was %s", snapshotNames[header.Type], snapshotNames[Full])
		}

		if len(targetNetworkID) > 0 {
			fullHeaderProtoParams, err := header.ProtocolParameters()
			if err != nil {
				return err
			}

			if fullHeaderProtoParams.NetworkID() != targetNetworkID[0] {
				return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file network ID is %d but this HORNET is meant for %d", fullHeaderProtoParams.NetworkID(), targetNetworkID[0])
			}
		}

		*targetFullHeader = *header

		if err := utxoManager.StoreLedgerIndex(header.LedgerMilestoneIndex); err != nil {
			return err
		}

		return nil
	}
}

// returns a file header consumer, which stores the ledger milestone index up on execution in the database.
// the given targetHeader is populated with the value of the read file header.
func newDeltaHeaderConsumer(targetHeader *DeltaSnapshotHeader) DeltaHeaderConsumerFunc {
	return func(header *DeltaSnapshotHeader) error {
		if header.Version != SupportedFormatVersion {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file version is %d but this HORNET version only supports %v", header.Version, SupportedFormatVersion)
		}

		if header.Type != Delta {
			return errors.Wrapf(ErrUnsupportedSnapshot, "snapshot file is of type %s but expected was %s", snapshotNames[header.Type], snapshotNames[Delta])
		}

		*targetHeader = *header

		return nil
	}
}

// returns a solid entry point consumer which stores them into the database.
// the SEPs are stored with the corresponding target milestone index from the snapshot.
func newSEPsConsumer(dbStorage *storage.Storage) SEPConsumerFunc {
	// note that we only get the hash of the SEP block instead
	// of also its associated oldest cone root index, since the index
	// of the snapshot milestone will be below max depth anyway.
	// this information was included in pre Chrysalis Phase 2 snapshots
	// but has been deemed unnecessary for the reason mentioned above.
	return func(solidEntryPointBlockID iotago.BlockID, targetMilestoneIndex iotago.MilestoneIndex) error {
		dbStorage.SolidEntryPointsAddWithoutLocking(solidEntryPointBlockID, targetMilestoneIndex)

		return nil
	}
}

// returns a ProtocolParamsMilestoneOpt consumer storing them into the database.
func newProtocolParamsMilestoneOptConsumerFunc(dbStorage *storage.Storage) ProtocolParamsMilestoneOptConsumerFunc {
	return func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) error {
		return dbStorage.StoreProtocolParametersMilestoneOption(protoParamsMsOption)
	}
}

// NewOutputConsumer returns an output consumer storing them into the database.
func NewOutputConsumer(utxoManager *utxo.Manager) OutputConsumerFunc {
	return utxoManager.AddUnspentOutput
}

// NewUnspentTreasuryOutputConsumer returns a treasury output consumer which overrides an existing unspent treasury output with the new one.
func NewUnspentTreasuryOutputConsumer(utxoManager *utxo.Manager) UnspentTreasuryOutputConsumerFunc {
	// leave like this for now in case we need to do more in the future
	return utxoManager.StoreUnspentTreasuryOutput
}

// NewMsDiffConsumer creates a milestone diff consumer storing them into the database.
// if the ledger index within the database equals the produced milestone diff's index,
// then its changes are roll-backed, otherwise, if the index is higher than the ledger index,
// its mutations are applied on top of the latest state.
// the caller needs to make sure to set the ledger index accordingly beforehand.
func NewMsDiffConsumer(dbStorage *storage.Storage, utxoManager *utxo.Manager, writeMilestonesToStorage bool) MilestoneDiffConsumerFunc {
	return func(msDiff *MilestoneDiff) error {

		if writeMilestonesToStorage {
			cachedMilestone, _ := dbStorage.StoreMilestoneIfAbsent(msDiff.Milestone) // milestone +1
			cachedMilestone.Release(true)                                            // milestone -1
		}

		msIndex := msDiff.Milestone.Index
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
			return errors.Wrapf(ErrWrongMilestoneDiffIndex, "ledgerIndex: %d, msDiffIndex: %d", ledgerIndex, msIndex)
		}
	}
}

// loadFullSnapshotFileToStorage loads a snapshot file from the given file path into the storage.
func loadFullSnapshotFileToStorage(
	ctx context.Context,
	dbStorage *storage.Storage,
	filePath string,
	targetNetworkID uint64,
	writeMilestonesToStorage bool) (fullHeader *FullSnapshotHeader, err error) {

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
		return nil, fmt.Errorf("unable to open %s snapshot file for import: %w", snapshotNames[Full], err)
	}
	defer func() { _ = lsFile.Close() }()

	fullHeader = &FullSnapshotHeader{}
	fullHeaderConsumer := newFullHeaderConsumer(fullHeader, dbStorage.UTXOManager(), targetNetworkID)
	treasuryOutputConsumer := NewUnspentTreasuryOutputConsumer(dbStorage.UTXOManager())
	outputConsumer := NewOutputConsumer(dbStorage.UTXOManager())
	msDiffConsumer := NewMsDiffConsumer(dbStorage, dbStorage.UTXOManager(), writeMilestonesToStorage)
	sepConsumer := newSEPsConsumer(dbStorage)
	protocolParamsMilestoneOptConsumer := newProtocolParamsMilestoneOptConsumerFunc(dbStorage)

	if err = StreamFullSnapshotDataFrom(
		ctx,
		lsFile,
		fullHeaderConsumer,
		treasuryOutputConsumer,
		outputConsumer,
		msDiffConsumer,
		sepConsumer,
		protocolParamsMilestoneOptConsumer); err != nil {
		return nil, fmt.Errorf("unable to import %s snapshot file: %w", snapshotNames[Full], err)
	}

	fullHeaderProtoParams, err := fullHeader.ProtocolParameters()
	if err != nil {
		return nil, err
	}

	if fullHeaderProtoParams.NetworkID() != targetNetworkID {
		return nil, fmt.Errorf("node is configured to operate for networkID %d but the stored snapshot data corresponds to %d", targetNetworkID, fullHeaderProtoParams.NetworkID())
	}

	if err := dbStorage.CheckLedgerState(); err != nil {
		return nil, err
	}

	var ledgerIndex iotago.MilestoneIndex
	ledgerIndex, err = dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, err
	}

	if ledgerIndex != fullHeader.TargetMilestoneIndex {
		return nil, errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchTargetIndex, "%d != %d", ledgerIndex, fullHeader.TargetMilestoneIndex)
	}

	if err = dbStorage.SetInitialSnapshotInfo(fullHeader.GenesisMilestoneIndex, fullHeader.TargetMilestoneIndex, fullHeader.TargetMilestoneIndex, fullHeader.TargetMilestoneIndex, time.Unix(int64(fullHeader.TargetMilestoneTimestamp), 0)); err != nil {
		return nil, fmt.Errorf("SetSnapshotMilestone failed: %w", err)
	}

	return fullHeader, nil
}

// loadSnapshotFileToStorage loads a snapshot file from the given file path into the storage.
// The current milestone index of the protocol manager must be set to the
// target index of the full snapshot file before entering this function.
func loadDeltaSnapshotFileToStorage(
	ctx context.Context,
	dbStorage *storage.Storage,
	filePath string,
	writeMilestonesToStorage bool) (deltaHeader *DeltaSnapshotHeader, err error) {

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
		return nil, fmt.Errorf("unable to open %s snapshot file for import: %w", snapshotNames[Delta], err)
	}
	defer func() { _ = lsFile.Close() }()

	deltaHeader = &DeltaSnapshotHeader{}
	protocolStorageGetter := newProtocolStorageGetterFunc(dbStorage)
	deltaHeaderConsumer := newDeltaHeaderConsumer(deltaHeader)
	msDiffConsumer := NewMsDiffConsumer(dbStorage, dbStorage.UTXOManager(), writeMilestonesToStorage)
	sepConsumer := newSEPsConsumer(dbStorage)
	protocolParamsMilestoneOptConsumer := newProtocolParamsMilestoneOptConsumerFunc(dbStorage)

	if err = StreamDeltaSnapshotDataFrom(
		ctx,
		lsFile,
		protocolStorageGetter,
		deltaHeaderConsumer,
		msDiffConsumer,
		sepConsumer,
		protocolParamsMilestoneOptConsumer); err != nil {
		return nil, fmt.Errorf("unable to import %s snapshot file: %w", snapshotNames[Delta], err)
	}

	if err := dbStorage.CheckLedgerState(); err != nil {
		return nil, err
	}

	var ledgerIndex iotago.MilestoneIndex
	ledgerIndex, err = dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, err
	}

	if ledgerIndex != deltaHeader.TargetMilestoneIndex {
		return nil, errors.Wrapf(ErrFinalLedgerIndexDoesNotMatchTargetIndex, "%d != %d", ledgerIndex, deltaHeader.TargetMilestoneIndex)
	}

	if err = dbStorage.UpdateSnapshotInfo(deltaHeader.TargetMilestoneIndex, deltaHeader.TargetMilestoneIndex, deltaHeader.TargetMilestoneIndex, time.Unix(int64(deltaHeader.TargetMilestoneTimestamp), 0)); err != nil {
		return nil, fmt.Errorf("SetSnapshotMilestone failed: %w", err)
	}

	return deltaHeader, nil
}

// LoadSnapshotFilesToStorage loads the snapshot files from the given file paths into the storage.
func LoadSnapshotFilesToStorage(ctx context.Context, dbStorage *storage.Storage, writeMilestonesToStorage bool, fullPath string, deltaPath ...string) (*FullSnapshotHeader, *DeltaSnapshotHeader, error) {

	fullHeader, err := ReadFullSnapshotHeaderFromFile(fullPath)
	if err != nil {
		return nil, nil, err
	}

	if len(deltaPath) > 0 && deltaPath[0] != "" {
		// check that the delta snapshot file's ledger index equals the snapshot index of the full one

		deltaHeader, err := ReadDeltaSnapshotHeaderFromFile(deltaPath[0])
		if err != nil {
			return nil, nil, err
		}

		if deltaHeader.FullSnapshotTargetMilestoneID != fullHeader.TargetMilestoneID {
			// delta snapshot file doesn't fit the full snapshot file
			return nil, nil, fmt.Errorf("%w: full snapshot target milestone ID of the delta snapshot does not fit the actual full snapshot target milestone ID (%s != %s)", ErrSnapshotsNotMergeable, deltaHeader.FullSnapshotTargetMilestoneID.ToHex(), fullHeader.TargetMilestoneID.ToHex())
		}
	}

	fullHeaderProtoParams, err := fullHeader.ProtocolParameters()
	if err != nil {
		return nil, nil, err
	}

	var fullSnapshotHeader *FullSnapshotHeader
	var deltaSnapshotHeader *DeltaSnapshotHeader
	fullSnapshotHeader, err = loadFullSnapshotFileToStorage(ctx, dbStorage, fullPath, fullHeaderProtoParams.NetworkID(), writeMilestonesToStorage)
	if err != nil {
		return nil, nil, err
	}

	if len(deltaPath) > 0 && deltaPath[0] != "" {
		deltaSnapshotHeader, err = loadDeltaSnapshotFileToStorage(ctx, dbStorage, deltaPath[0], writeMilestonesToStorage)
		if err != nil {
			return nil, nil, err
		}
	}

	return fullSnapshotHeader, deltaSnapshotHeader, nil
}
