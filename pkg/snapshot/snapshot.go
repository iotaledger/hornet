package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iotaledger/hornet/pkg/protocol"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/model/utxo"
)

var (
	// ErrUnsupportedSnapshot is returned when unsupported snapshot data is read.
	ErrUnsupportedSnapshot = errors.New("unsupported snapshot data")
	// ErrWrongMilestoneDiffIndex is returned when the milestone diff that should be applied is not the current or next milestone.
	ErrWrongMilestoneDiffIndex = errors.New("wrong milestone diff index")
	// ErrFinalLedgerIndexDoesNotMatchSEPIndex is returned when the final milestone after loading the snapshot is not equal to the solid entry point index.
	ErrFinalLedgerIndexDoesNotMatchSEPIndex = errors.New("final ledger index does not match solid entry point index")
	// ErrInvalidSnapshotAvailabilityState is returned when a delta snapshot is available, but no full snapshot is found.
	ErrInvalidSnapshotAvailabilityState = errors.New("invalid snapshot files availability")
	// ErrNoMoreSEPToProduce is returned when there are no more solid entry points to produce.
	ErrNoMoreSEPToProduce = errors.New("no more SEP to produce")

	ErrNoSnapshotSpecified                   = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL                 = errors.New("no download URL specified for snapshot files in config")
	ErrSnapshotDownloadWasAborted            = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource         = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotCreationWasAborted            = errors.New("operation was aborted")
	ErrSnapshotCreationFailed                = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew                     = errors.New("snapshot target is too new")
	ErrTargetIndexTooOld                     = errors.New("snapshot target is too old")
	ErrNotEnoughHistory                      = errors.New("not enough history")
	ErrExistingDeltaSnapshotWrongLedgerIndex = errors.New("existing delta ledger snapshot has wrong ledger index")
)

type snapshotAvailability byte

const (
	snapshotAvailBoth snapshotAvailability = iota
	snapshotAvailOnlyFull
	snapshotAvailNone
)

// Manager handles reading and writing snapshot data.
type Manager struct {
	// the logger used to log events.
	*logger.WrappedLogger

	storage                              *storage.Storage
	syncManager                          *syncmanager.SyncManager
	utxoManager                          *utxo.Manager
	protoMng                             *protocol.Manager
	snapshotFullPath                     string
	snapshotDeltaPath                    string
	deltaSnapshotSizeThresholdPercentage float64
	downloadTargets                      []*DownloadTarget
	solidEntryPointCheckThresholdPast    milestone.Index
	solidEntryPointCheckThresholdFuture  milestone.Index
	snapshotDepth                        milestone.Index
	snapshotInterval                     milestone.Index

	snapshotLock         syncutils.Mutex
	statusLock           syncutils.RWMutex
	statusIsSnapshotting bool

	Events *Events
}

// NewSnapshotManager creates a new snapshot manager instance.
func NewSnapshotManager(
	log *logger.Logger,
	storage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	utxoManager *utxo.Manager,
	protoMng *protocol.Manager,
	snapshotFullPath string,
	snapshotDeltaPath string,
	deltaSnapshotSizeThresholdPercentage float64,
	downloadTargets []*DownloadTarget,
	solidEntryPointCheckThresholdPast milestone.Index,
	solidEntryPointCheckThresholdFuture milestone.Index,
	additionalPruningThreshold milestone.Index,
	snapshotDepth milestone.Index,
	snapshotInterval milestone.Index,
) *Manager {

	return &Manager{
		WrappedLogger:                        logger.NewWrappedLogger(log),
		storage:                              storage,
		syncManager:                          syncManager,
		utxoManager:                          utxoManager,
		protoMng:                             protoMng,
		snapshotFullPath:                     snapshotFullPath,
		snapshotDeltaPath:                    snapshotDeltaPath,
		deltaSnapshotSizeThresholdPercentage: deltaSnapshotSizeThresholdPercentage,
		downloadTargets:                      downloadTargets,
		solidEntryPointCheckThresholdPast:    solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture:  solidEntryPointCheckThresholdFuture,
		snapshotDepth:                        snapshotDepth,
		snapshotInterval:                     snapshotInterval,
		Events: &Events{
			SnapshotMilestoneIndexChanged:         events.NewEvent(milestone.IndexCaller),
			HandledConfirmedMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
			SnapshotMetricsUpdated:                events.NewEvent(SnapshotMetricsCaller),
		},
	}
}

func (s *Manager) MinimumMilestoneIndex() milestone.Index {

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		s.LogPanic("No snapshotInfo found!")
		return 0
	}

	minimumIndex := snapshotInfo.SnapshotIndex
	minimumIndex -= s.snapshotDepth
	minimumIndex -= s.solidEntryPointCheckThresholdPast

	return minimumIndex
}

func (s *Manager) IsSnapshotting() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.statusIsSnapshotting
}

func (s *Manager) shouldTakeSnapshot(confirmedMilestoneIndex milestone.Index) bool {

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		s.LogPanic("No snapshotInfo found!")
		return false
	}

	if (confirmedMilestoneIndex < s.snapshotDepth+s.snapshotInterval) || (confirmedMilestoneIndex-s.snapshotDepth) < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return confirmedMilestoneIndex-(s.snapshotDepth+s.snapshotInterval) >= snapshotInfo.SnapshotIndex
}

func checkSnapshotLimits(
	snapshotInfo *storage.SnapshotInfo,
	confirmedMilestoneIndex milestone.Index,
	targetIndex milestone.Index,
	solidEntryPointCheckThresholdPast milestone.Index,
	solidEntryPointCheckThresholdFuture milestone.Index,
	checkIncreasingSnapshotIndex bool) error {

	if confirmedMilestoneIndex < solidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum confirmed index: %d, actual confirmed index: %d", solidEntryPointCheckThresholdFuture+1, confirmedMilestoneIndex)
	}

	minimumIndex := solidEntryPointCheckThresholdPast + 1
	maximumIndex := confirmedMilestoneIndex - solidEntryPointCheckThresholdFuture

	if checkIncreasingSnapshotIndex && minimumIndex < snapshotInfo.SnapshotIndex+1 {
		minimumIndex = snapshotInfo.SnapshotIndex + 1
	}

	if minimumIndex < snapshotInfo.PruningIndex+1+solidEntryPointCheckThresholdPast {
		// since we always generate new solid entry points, we need enough history
		minimumIndex = snapshotInfo.PruningIndex + 1 + solidEntryPointCheckThresholdPast
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

func (s *Manager) setIsSnapshotting(value bool) {
	s.statusLock.Lock()
	s.statusIsSnapshotting = value
	s.statusLock.Unlock()
}

// CreateFullSnapshot creates a full snapshot for the given target milestone index.
func (s *Manager) CreateFullSnapshot(ctx context.Context, targetIndex milestone.Index, filePath string, writeToDatabase bool) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createSnapshotWithoutLocking(ctx, Full, targetIndex, filePath, writeToDatabase)
}

// CreateDeltaSnapshot creates a delta snapshot for the given target milestone index.
func (s *Manager) CreateDeltaSnapshot(ctx context.Context, targetIndex milestone.Index, filePath string, writeToDatabase bool, snapshotFullPath ...string) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createSnapshotWithoutLocking(ctx, Delta, targetIndex, filePath, writeToDatabase, snapshotFullPath...)
}

// LoadSnapshotFromFile loads a snapshot file from the given file path into the storage.
func (s *Manager) LoadSnapshotFromFile(ctx context.Context, snapshotType Type, filePath string) (err error) {
	s.LogInfof("importing %s snapshot file...", snapshotNames[snapshotType])
	ts := time.Now()

	header, err := loadSnapshotFileToStorage(ctx, s.storage, snapshotType, filePath, s.protoMng.Current())
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

// optimalSnapshotType returns the optimal snapshot type
// based on the file size of the last full and delta snapshot file.
func (s *Manager) optimalSnapshotType() (Type, error) {
	if s.deltaSnapshotSizeThresholdPercentage == 0.0 {
		// special case => always create a delta snapshot to keep entire milestone diff history
		return Delta, nil
	}

	fullSnapshotFileInfo, err := os.Stat(s.snapshotFullPath)
	fullSnapshotFileExists := !os.IsNotExist(err)

	if !fullSnapshotFileExists {
		// full snapshot doesn't exist => create a full snapshot
		return Full, nil
	}

	if err != nil {
		// there was another unknown error
		return Full, err
	}

	deltaSnapshotFileInfo, err := os.Stat(s.snapshotDeltaPath)
	deltaSnapshotFileExists := !os.IsNotExist(err)

	if !deltaSnapshotFileExists {
		// delta snapshot doesn't exist => create a delta snapshot
		return Delta, nil
	}

	if err != nil {
		// there was another unknown error
		return Delta, err
	}

	// if the file size of the last delta snapshot is bigger than a certain percentage
	// of the full snapshot file, it's more efficient to create a new full snapshot.
	if int64(float64(fullSnapshotFileInfo.Size())*s.deltaSnapshotSizeThresholdPercentage/100.0) < deltaSnapshotFileInfo.Size() {
		return Full, nil
	}

	return Delta, nil
}

// snapshotTypeFilePath returns the default file path
// for the given snapshot type.
func (s *Manager) snapshotTypeFilePath(snapshotType Type) string {
	switch snapshotType {
	case Full:
		return s.snapshotFullPath
	case Delta:
		return s.snapshotDeltaPath
	default:
		panic("unknown snapshot type")
	}
}

// HandleNewConfirmedMilestoneEvent handles new confirmed milestone events which may trigger a snapshot creation.
func (s *Manager) HandleNewConfirmedMilestoneEvent(ctx context.Context, confirmedMilestoneIndex milestone.Index) {
	if !s.syncManager.IsNodeSynced() {
		// do not prune or create snapshots while we are not synced
		return
	}

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	if s.shouldTakeSnapshot(confirmedMilestoneIndex) {
		snapshotType, err := s.optimalSnapshotType()
		if err != nil {
			s.LogWarnf("%s: %s", ErrSnapshotCreationFailed, err)
			return
		}

		if err := s.createSnapshotWithoutLocking(ctx, snapshotType, confirmedMilestoneIndex-s.snapshotDepth, s.snapshotTypeFilePath(snapshotType), true); err != nil {
			if errors.Is(err, common.ErrCritical) {
				s.LogPanicf("%s: %s", ErrSnapshotCreationFailed, err)
			}
			s.LogWarnf("%s: %s", ErrSnapshotCreationFailed, err)
		}
	}

	s.Events.HandledConfirmedMilestoneIndexChanged.Trigger(confirmedMilestoneIndex)
}

// SnapshotsFilesLedgerIndex returns the final ledger index if the snapshots from the configured file paths would be applied.
func (s *Manager) SnapshotsFilesLedgerIndex() (milestone.Index, error) {

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

// ImportSnapshots imports snapshot data from the configured file paths.
// automatically downloads snapshot data if no files are available.
func (s *Manager) ImportSnapshots(ctx context.Context) error {
	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return err
	}

	if snapAvail == snapshotAvailNone {
		if err = s.downloadSnapshotFiles(ctx, s.protoMng.Current().NetworkID(), s.snapshotFullPath, s.snapshotDeltaPath); err != nil {
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
func (s *Manager) checkSnapshotFilesAvailability(fullPath string, deltaPath string) (snapshotAvailability, error) {
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
func (s *Manager) downloadSnapshotFiles(ctx context.Context, wantedNetworkID uint64, fullPath string, deltaPath string) error {
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

// CheckCurrentSnapshot checks that the current snapshot info is valid regarding its network ID and the ledger state.
func (s *Manager) CheckCurrentSnapshot(snapshotInfo *storage.SnapshotInfo) error {

	// check that the stored snapshot corresponds to the wanted network ID
	protoParas := s.protoMng.Current()
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
