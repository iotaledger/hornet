package snapshot

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	storagepkg "github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	// ErrUnsupportedSnapshot is returned when unsupported snapshot data is read.
	ErrUnsupportedSnapshot = errors.New("unsupported snapshot data")
	// ErrWrongMilestoneDiffIndex is returned when the milestone diff that should be applied is not the current or next milestone.
	ErrWrongMilestoneDiffIndex = errors.New("wrong milestone diff index")
	// ErrFinalLedgerIndexDoesNotMatchTargetIndex is returned when the final milestone after loading the snapshot is not equal to the target index.
	ErrFinalLedgerIndexDoesNotMatchTargetIndex = errors.New("final ledger index does not match target index")
	// ErrInvalidSnapshotAvailabilityState is returned when a delta snapshot is available, but no full snapshot is found.
	ErrInvalidSnapshotAvailabilityState = errors.New("invalid snapshot files availability")
	// ErrDeltaSnapshotIncompatible is returned when a delta snapshot file does not match full snapshot file.
	ErrDeltaSnapshotIncompatible = errors.New("delta snapshot file does not match full snapshot file")
	// ErrNoMoreSEPToProduce is returned when there are no more solid entry points to produce.
	ErrNoMoreSEPToProduce = errors.New("no more SEP to produce")

	ErrNoSnapshotSpecified           = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL         = errors.New("no download URL specified for snapshot files in config")
	ErrSnapshotDownloadWasAborted    = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotCreationWasAborted    = errors.New("operation was aborted")
	ErrSnapshotCreationFailed        = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew             = errors.New("snapshot target is too new")
	ErrTargetIndexTooOld             = errors.New("snapshot target is too old")
	ErrNotEnoughHistory              = errors.New("not enough history")
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

	storage                                *storagepkg.Storage
	syncManager                            *syncmanager.SyncManager
	utxoManager                            *utxo.Manager
	snapshotCreationEnabled                bool
	snapshotFullPath                       string
	snapshotDeltaPath                      string
	deltaSnapshotSizeThresholdPercentage   float64
	deltaSnapshotSizeThresholdMinSizeBytes int64
	solidEntryPointCheckThresholdPast      syncmanager.MilestoneIndexDelta
	solidEntryPointCheckThresholdFuture    syncmanager.MilestoneIndexDelta
	snapshotDepth                          syncmanager.MilestoneIndexDelta
	snapshotInterval                       syncmanager.MilestoneIndexDelta

	snapshotLock         syncutils.Mutex
	statusLock           syncutils.RWMutex
	statusIsSnapshotting bool

	Events *Events
}

// NewSnapshotManager creates a new snapshot manager instance.
func NewSnapshotManager(
	log *logger.Logger,
	storage *storagepkg.Storage,
	syncManager *syncmanager.SyncManager,
	utxoManager *utxo.Manager,
	snapshotCreationEnabled bool,
	snapshotFullPath string,
	snapshotDeltaPath string,
	deltaSnapshotSizeThresholdPercentage float64,
	deltaSnapshotSizeThresholdMinSizeBytes int64,
	solidEntryPointCheckThresholdPast syncmanager.MilestoneIndexDelta,
	solidEntryPointCheckThresholdFuture syncmanager.MilestoneIndexDelta,
	snapshotDepth syncmanager.MilestoneIndexDelta,
	snapshotInterval iotago.MilestoneIndex,
) *Manager {

	return &Manager{
		WrappedLogger:                          logger.NewWrappedLogger(log),
		storage:                                storage,
		syncManager:                            syncManager,
		utxoManager:                            utxoManager,
		snapshotCreationEnabled:                snapshotCreationEnabled,
		snapshotFullPath:                       snapshotFullPath,
		snapshotDeltaPath:                      snapshotDeltaPath,
		deltaSnapshotSizeThresholdPercentage:   deltaSnapshotSizeThresholdPercentage,
		deltaSnapshotSizeThresholdMinSizeBytes: deltaSnapshotSizeThresholdMinSizeBytes,
		solidEntryPointCheckThresholdPast:      solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture:    solidEntryPointCheckThresholdFuture,
		snapshotDepth:                          snapshotDepth,
		snapshotInterval:                       snapshotInterval,
		Events: &Events{
			SnapshotMilestoneIndexChanged:         events.NewEvent(storagepkg.MilestoneIndexCaller),
			HandledConfirmedMilestoneIndexChanged: events.NewEvent(storagepkg.MilestoneIndexCaller),
			SnapshotMetricsUpdated:                events.NewEvent(MetricsCaller),
		},
	}
}

func (s *Manager) MinimumMilestoneIndex() iotago.MilestoneIndex {
	minimumIndex := s.syncManager.ConfirmedMilestoneIndex()

	if s.snapshotCreationEnabled {
		snapshotInfo := s.storage.SnapshotInfo()
		if snapshotInfo == nil {
			s.LogPanic(common.ErrSnapshotInfoNotFound)

			return 0
		}

		minimumIndex = snapshotInfo.SnapshotIndex()
		if minimumIndex < s.snapshotDepth {
			return 0
		}
		minimumIndex -= s.snapshotDepth
	}

	if minimumIndex < s.solidEntryPointCheckThresholdPast {
		return 0
	}
	minimumIndex -= s.solidEntryPointCheckThresholdPast

	return minimumIndex
}

func (s *Manager) IsSnapshotting() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()

	return s.statusIsSnapshotting
}

func (s *Manager) shouldTakeSnapshot(confirmedMilestoneIndex iotago.MilestoneIndex) bool {
	if !s.snapshotCreationEnabled {
		return false
	}

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		s.LogPanic(common.ErrSnapshotInfoNotFound)

		return false
	}

	if (confirmedMilestoneIndex < s.snapshotDepth+s.snapshotInterval) || (confirmedMilestoneIndex-s.snapshotDepth) < snapshotInfo.PruningIndex()+1+s.solidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return confirmedMilestoneIndex-(s.snapshotDepth+s.snapshotInterval) >= snapshotInfo.SnapshotIndex()
}

func checkSnapshotLimits(
	snapshotInfo *storagepkg.SnapshotInfo,
	confirmedMilestoneIndex iotago.MilestoneIndex,
	targetIndex iotago.MilestoneIndex,
	globalSnapshot bool,
	solidEntryPointCheckThresholdPast syncmanager.MilestoneIndexDelta,
	solidEntryPointCheckThresholdFuture syncmanager.MilestoneIndexDelta,
	checkIncreasingSnapshotIndex bool) error {

	var minimumIndex uint32
	var maximumIndex uint32

	if globalSnapshot {
		// if we create a global snapshot, we do not need to calculate the SEP.
		// we can simply take the milestone parents of the ledger milestone.
		minimumIndex = snapshotInfo.PruningIndex() + 1
		maximumIndex = confirmedMilestoneIndex

	} else {
		if confirmedMilestoneIndex < solidEntryPointCheckThresholdFuture {
			return errors.Wrapf(ErrNotEnoughHistory, "minimum confirmed index: %d, actual confirmed index: %d", solidEntryPointCheckThresholdFuture+1, confirmedMilestoneIndex)
		}

		minimumIndex = solidEntryPointCheckThresholdPast + 1
		maximumIndex = confirmedMilestoneIndex - solidEntryPointCheckThresholdFuture

		if checkIncreasingSnapshotIndex && minimumIndex < snapshotInfo.SnapshotIndex()+1 {
			minimumIndex = snapshotInfo.SnapshotIndex() + 1
		}

		if minimumIndex < snapshotInfo.PruningIndex()+1+solidEntryPointCheckThresholdPast {
			// since we always generate new solid entry points, we need enough history
			minimumIndex = snapshotInfo.PruningIndex() + 1 + solidEntryPointCheckThresholdPast
		}
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
func (s *Manager) CreateFullSnapshot(ctx context.Context, targetIndex iotago.MilestoneIndex, filePath string, writeToDatabase bool) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	return s.createFullSnapshotWithoutLocking(ctx, targetIndex, filePath, writeToDatabase)
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

	// both files exist => check the size of the delta snapshot
	// if the size of the delta snapshot is smaller than the minimum threshold,
	// the existing delta snapshot file always gets updated.
	if deltaSnapshotFileInfo.Size() <= s.deltaSnapshotSizeThresholdMinSizeBytes {
		return Delta, nil
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
func (s *Manager) HandleNewConfirmedMilestoneEvent(ctx context.Context, confirmedMilestoneIndex iotago.MilestoneIndex) {
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

		switch snapshotType {
		case Full:
			err = s.createFullSnapshotWithoutLocking(ctx, confirmedMilestoneIndex-s.snapshotDepth, s.snapshotTypeFilePath(snapshotType), true)
		case Delta:
			err = s.createDeltaSnapshotWithoutLocking(ctx, confirmedMilestoneIndex-s.snapshotDepth)
		}

		if err != nil {
			if errors.Is(err, common.ErrCritical) {
				s.LogPanicf("%s: %s", ErrSnapshotCreationFailed, err)
			}
			s.LogWarnf("%s: %s", ErrSnapshotCreationFailed, err)
		}
	}

	s.Events.HandledConfirmedMilestoneIndexChanged.Trigger(confirmedMilestoneIndex)
}

func FormatSnapshotTimestamp(timestamp uint32) string {
	result := "unknown"
	if timestamp != 0 {
		result = time.Unix(int64(timestamp), 0).Truncate(time.Second).String()
	}

	return result
}
