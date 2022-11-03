package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Importer struct {
	// the logger used to log events.
	*logger.WrappedLogger

	storage           *storage.Storage
	snapshotFullPath  string
	snapshotDeltaPath string
	targetNetworkName string
	downloadTargets   []*DownloadTarget
}

// NewSnapshotImporter creates a new snapshot manager instance.
func NewSnapshotImporter(
	log *logger.Logger,
	storage *storage.Storage,
	snapshotFullPath string,
	snapshotDeltaPath string,
	targetNetworkName string,
	downloadTargets []*DownloadTarget) *Importer {

	return &Importer{
		WrappedLogger:     logger.NewWrappedLogger(log),
		storage:           storage,
		snapshotFullPath:  snapshotFullPath,
		snapshotDeltaPath: snapshotDeltaPath,
		targetNetworkName: targetNetworkName,
		downloadTargets:   downloadTargets,
	}
}

// ImportSnapshots imports snapshot data from the configured file paths.
// automatically downloads snapshot data if no files are available.
func (s *Importer) ImportSnapshots(ctx context.Context) error {
	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return err
	}

	targetNetworkID := iotago.NetworkIDFromString(s.targetNetworkName)

	if snapAvail == snapshotAvailNone {
		if err = s.downloadSnapshotFiles(ctx, targetNetworkID, s.snapshotFullPath, s.snapshotDeltaPath); err != nil {
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

	if err = s.LoadFullSnapshotFromFile(ctx, s.snapshotFullPath, targetNetworkID); err != nil {
		_ = s.storage.MarkStoresCorrupted()

		return err
	}

	if snapAvail == snapshotAvailOnlyFull {
		return nil
	}

	if err = s.LoadDeltaSnapshotFromFile(ctx, s.snapshotDeltaPath); err != nil {
		_ = s.storage.MarkStoresCorrupted()

		return err
	}

	return nil
}

// checks that either both snapshot files are available, only the full snapshot or none.
func (s *Importer) checkSnapshotFilesAvailability(fullPath string, deltaPath string) (snapshotAvailability, error) {
	if len(fullPath) == 0 {
		return 0, fmt.Errorf("%w: full snapshot file path not defined", ErrNoSnapshotSpecified)
	}

	_, fullSnapshotStatErr := os.Stat(fullPath)
	if len(deltaPath) == 0 {
		// no delta path specified, check if full snapshot is available
		if os.IsNotExist(fullSnapshotStatErr) {
			return snapshotAvailNone, nil
		}

		return snapshotAvailOnlyFull, nil
	}

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
func (s *Importer) downloadSnapshotFiles(ctx context.Context, targetNetworkID uint64, fullPath string, deltaPath string) error {
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

	if err := s.DownloadSnapshotFiles(ctx, targetNetworkID, fullPath, deltaPath, s.downloadTargets); err != nil {
		return fmt.Errorf("unable to download snapshot files: %w", err)
	}

	s.LogInfo("snapshot download finished")

	return nil
}

// LoadFullSnapshotFromFile loads a snapshot file from the given file path into the storage.
func (s *Importer) LoadFullSnapshotFromFile(ctx context.Context, filePath string, targetNetworkID iotago.NetworkID) (err error) {
	snapshotName := snapshotNames[Full]

	s.LogInfof("importing %s snapshot file ...", snapshotName)
	ts := time.Now()

	fullHeader, err := loadFullSnapshotFileToStorage(ctx, s.storage, filePath, targetNetworkID, false)
	if err != nil {
		return err
	}

	protoParams, err := s.storage.ProtocolParameters(fullHeader.LedgerMilestoneIndex)
	if err != nil {
		return fmt.Errorf("loading protocol parameters failed: %w", err)
	}

	s.LogInfof("imported %s snapshot file, took %v", snapshotName, time.Since(ts).Truncate(time.Millisecond))
	s.LogInfof("solid entry points: %d, outputs: %d, ms diffs: %d", fullHeader.SEPCount, fullHeader.OutputCount, fullHeader.MilestoneDiffCount)
	s.LogInfof(`
SnapshotInfo:
    Type:            %s
    NetworkID:       %d
    SnapshotIndex:   %d
    EntryPointIndex: %d
    PruningIndex:    %d
    Timestamp:       %s`,

		snapshotName,
		protoParams.NetworkID(),
		fullHeader.TargetMilestoneIndex,
		fullHeader.TargetMilestoneIndex,
		fullHeader.TargetMilestoneIndex,
		FormatSnapshotTimestamp(fullHeader.TargetMilestoneTimestamp))

	return nil
}

// LoadDeltaSnapshotFromFile loads a snapshot file from the given file path into the storage.
func (s *Importer) LoadDeltaSnapshotFromFile(ctx context.Context, filePath string) (err error) {
	snapshotName := snapshotNames[Delta]

	s.LogInfof("importing %s snapshot file ...", snapshotName)
	ts := time.Now()

	header, err := loadDeltaSnapshotFileToStorage(ctx, s.storage, filePath, false)
	if err != nil {
		return err
	}

	protoParams, err := s.storage.ProtocolParameters(header.TargetMilestoneIndex)
	if err != nil {
		return fmt.Errorf("loading protocol parameters failed: %w", err)
	}

	s.LogInfof("imported %s snapshot file, took %v", snapshotName, time.Since(ts).Truncate(time.Millisecond))
	s.LogInfof("solid entry points: %d, ms diffs: %d", header.SEPCount, header.MilestoneDiffCount)
	s.LogInfof(`
SnapshotInfo:
    Type:            %s
    NetworkID:       %d
    SnapshotIndex:   %d
    EntryPointIndex: %d
    PruningIndex:    %d
    Timestamp:       %s`,

		snapshotName,
		protoParams.NetworkID(),
		header.TargetMilestoneIndex,
		header.TargetMilestoneIndex,
		header.TargetMilestoneIndex,
		FormatSnapshotTimestamp(header.TargetMilestoneTimestamp),
	)

	return nil
}

// SnapshotsFilesLedgerIndex returns the final ledger index if the snapshots from the configured file paths would be applied.
func (s *Importer) SnapshotsFilesLedgerIndex() (iotago.MilestoneIndex, error) {

	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return 0, err
	}

	if snapAvail == snapshotAvailNone {
		return 0, errors.New("no snapshot files available")
	}

	fullHeader, err := ReadFullSnapshotHeaderFromFile(s.snapshotFullPath)
	if err != nil {
		return 0, err
	}

	var deltaHeader *DeltaSnapshotHeader
	if snapAvail == snapshotAvailBoth {
		deltaHeader, err = ReadDeltaSnapshotHeaderFromFile(s.snapshotDeltaPath)
		if err != nil {
			return 0, err
		}
	}

	return getSnapshotFilesLedgerIndex(fullHeader, deltaHeader), nil
}
