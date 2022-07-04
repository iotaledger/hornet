package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/protocol"
)

type SnapshotImporter struct {
	// the logger used to log events.
	*logger.WrappedLogger

	storage           *storage.Storage
	syncManager       *syncmanager.SyncManager
	utxoManager       *utxo.Manager
	protocolManager   *protocol.Manager
	snapshotFullPath  string
	snapshotDeltaPath string
	targetNetworkName string
	downloadTargets   []*DownloadTarget
}

// NewSnapshotImporter creates a new snapshot manager instance.
func NewSnapshotImporter(
	log *logger.Logger,
	storage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	utxoManager *utxo.Manager,
	protocolManager *protocol.Manager,
	snapshotFullPath string,
	snapshotDeltaPath string,
	targetNetworkName string,
	downloadTargets []*DownloadTarget) *SnapshotImporter {

	return &SnapshotImporter{
		WrappedLogger:     logger.NewWrappedLogger(log),
		storage:           storage,
		syncManager:       syncManager,
		utxoManager:       utxoManager,
		protocolManager:   protocolManager,
		snapshotFullPath:  snapshotFullPath,
		snapshotDeltaPath: snapshotDeltaPath,
		targetNetworkName: targetNetworkName,
		downloadTargets:   downloadTargets,
	}
}

// ImportSnapshots imports snapshot data from the configured file paths.
// automatically downloads snapshot data if no files are available.
func (s *SnapshotImporter) ImportSnapshots(ctx context.Context) error {
	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return err
	}

	if snapAvail == snapshotAvailNone {
		if err = s.downloadSnapshotFiles(ctx, s.protocolManager.Current().NetworkID(), s.snapshotFullPath, s.snapshotDeltaPath); err != nil {
			return err
		}
	}

	snapAvail, err = s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return err
	}

	if snapAvail == snapshotAvailNone {
		return errors.New("no snapshot files available after snapshot download")
	}

	if err = s.LoadSnapshotFromFile(ctx, Full, s.snapshotFullPath); err != nil {
		_ = s.storage.MarkDatabasesCorrupted()
		return err
	}

	if snapAvail == snapshotAvailOnlyFull {
		return nil
	}

	if err = s.LoadSnapshotFromFile(ctx, Delta, s.snapshotDeltaPath); err != nil {
		_ = s.storage.MarkDatabasesCorrupted()
		return err
	}

	return nil
}

// checks that either both snapshot files are available, only the full snapshot or none.
func (s *SnapshotImporter) checkSnapshotFilesAvailability(fullPath string, deltaPath string) (snapshotAvailability, error) {
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
func (s *SnapshotImporter) downloadSnapshotFiles(ctx context.Context, wantedNetworkID uint64, fullPath string, deltaPath string) error {
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

	targetsJSON, err := json.MarshalIndent(s.downloadTargets, "", "   ")
	if err != nil {
		return fmt.Errorf("unable to marshal targets into formatted JSON: %w", err)
	}
	s.LogInfof("downloading snapshot files from one of the provided sources %s", string(targetsJSON))

	if err := s.DownloadSnapshotFiles(ctx, wantedNetworkID, fullPath, deltaPath, s.downloadTargets); err != nil {
		return fmt.Errorf("unable to download snapshot files: %w", err)
	}

	s.LogInfo("snapshot download finished")
	return nil
}

// LoadSnapshotFromFile loads a snapshot file from the given file path into the storage.
func (s *SnapshotImporter) LoadSnapshotFromFile(ctx context.Context, snapshotType Type, filePath string) (err error) {
	s.LogInfof("importing %s snapshot file...", snapshotNames[snapshotType])
	ts := time.Now()

	header, err := loadSnapshotFileToStorage(ctx, s.storage, snapshotType, filePath, s.protocolManager.Current())
	if err != nil {
		return err
	}

	if err := s.syncManager.SetConfirmedMilestoneIndex(header.SEPMilestoneIndex, false); err != nil {
		return fmt.Errorf("SetConfirmedMilestoneIndex failed: %w", err)
	}

	s.LogInfof("imported %s snapshot file, took %v", snapshotNames[snapshotType], time.Since(ts).Truncate(time.Millisecond))
	s.LogInfof("solid entry points: %d, outputs: %d, ms diffs: %d", header.SEPCount, header.OutputCount, header.MilestoneDiffCount)
	s.LogInfof(`
SnapshotInfo:
	Type: %s
	NetworkID: %d
	SnapshotIndex: %d
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, snapshotNames[snapshotType], header.NetworkID, header.SEPMilestoneIndex, header.SEPMilestoneIndex, header.SEPMilestoneIndex, time.Unix(int64(header.Timestamp), 0))

	return nil
}

// SnapshotsFilesLedgerIndex returns the final ledger index if the snapshots from the configured file paths would be applied.
func (s *SnapshotImporter) SnapshotsFilesLedgerIndex() (milestone.Index, error) {

	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return 0, err
	}

	if snapAvail == snapshotAvailNone {
		return 0, errors.New("no snapshot files available")
	}

	fullHeader, err := ReadSnapshotHeaderFromFile(s.snapshotFullPath)
	if err != nil {
		return 0, err
	}

	var deltaHeader *ReadFileHeader
	if snapAvail == snapshotAvailBoth {
		deltaHeader, err = ReadSnapshotHeaderFromFile(s.snapshotDeltaPath)
		if err != nil {
			return 0, err
		}
	}

	return getSnapshotFilesLedgerIndex(fullHeader, deltaHeader), nil
}

// CheckCurrentSnapshot checks that the current snapshot info is valid regarding its network ID and the ledger state.
func (s *SnapshotImporter) CheckCurrentSnapshot(snapshotInfo *storage.SnapshotInfo) error {

	// check that the stored snapshot corresponds to the wanted network ID
	protoParas := s.protocolManager.Current()
	if snapshotInfo.NetworkID != protoParas.NetworkID() {
		s.LogPanicf("node is configured to operate in network %d/%s but the stored snapshot data corresponds to %d", protoParas.NetworkID(), protoParas.NetworkName, snapshotInfo.NetworkID)
	}

	// if we don't enforce loading of a snapshot,
	// we can check the ledger state of the current database and start the node.
	if err := s.utxoManager.CheckLedgerState(protoParas); err != nil {
		s.LogFatalAndExit(err)
	}

	return nil
}
