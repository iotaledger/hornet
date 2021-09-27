package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
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
	// Returned when a delta snapshot is available, but no full snapshot is found.
	ErrInvalidSnapshotAvailabilityState = errors.New("invalid snapshot files availability")

	ErrNoSnapshotSpecified                   = errors.New("no snapshot file was specified in the config")
	ErrNoSnapshotDownloadURL                 = errors.New("no download URL specified for snapshot files in config")
	ErrSnapshotDownloadWasAborted            = errors.New("snapshot download was aborted")
	ErrSnapshotDownloadNoValidSource         = errors.New("no valid source found, snapshot download not possible")
	ErrSnapshotCreationWasAborted            = errors.New("operation was aborted")
	ErrSnapshotCreationFailed                = errors.New("creating snapshot failed")
	ErrTargetIndexTooNew                     = errors.New("snapshot target is too new")
	ErrTargetIndexTooOld                     = errors.New("snapshot target is too old")
	ErrNotEnoughHistory                      = errors.New("not enough history")
	ErrNoPruningNeeded                       = errors.New("no pruning needed")
	ErrPruningAborted                        = errors.New("pruning was aborted")
	ErrDatabaseCompactionNotSupported        = errors.New("database compaction not supported")
	ErrDatabaseCompactionRunning             = errors.New("database compaction is running")
	ErrExistingDeltaSnapshotWrongLedgerIndex = errors.New("existing delta ledger snapshot has wrong ledger index")
)

type snapshotAvailability byte

const (
	snapshotAvailBoth snapshotAvailability = iota
	snapshotAvailOnlyFull
	snapshotAvailNone
)

// SnapshotManager handles reading and writing snapshot data.
type SnapshotManager struct {
	shutdownCtx                          context.Context
	log                                  *logger.Logger
	database                             *database.Database
	storage                              *storage.Storage
	syncManager                          *syncmanager.SyncManager
	utxoManager                          *utxo.Manager
	networkID                            uint64
	networkIDSource                      string
	snapshotFullPath                     string
	snapshotDeltaPath                    string
	deltaSnapshotSizeThresholdPercentage float64
	downloadTargets                      []*DownloadTarget
	solidEntryPointCheckThresholdPast    milestone.Index
	solidEntryPointCheckThresholdFuture  milestone.Index
	additionalPruningThreshold           milestone.Index
	snapshotDepth                        milestone.Index
	snapshotInterval                     milestone.Index
	pruningMilestonesEnabled             bool
	pruningMilestonesMaxMilestonesToKeep milestone.Index
	pruningSizeEnabled                   bool
	pruningSizeTargetSizeBytes           int64
	pruningSizeThresholdPercentage       float64
	pruningSizeCooldownTime              time.Duration
	pruneReceipts                        bool

	snapshotLock          syncutils.Mutex
	statusLock            syncutils.RWMutex
	isSnapshotting        bool
	isPruning             bool
	lastPruningBySizeTime time.Time

	Events *Events
}

// NewSnapshotManager creates a new snapshot manager instance.
func NewSnapshotManager(shutdownCtx context.Context,
	log *logger.Logger,
	database *database.Database,
	storage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	utxoManager *utxo.Manager,
	networkID uint64,
	networkIDSource string,
	snapshotFullPath string,
	snapshotDeltaPath string,
	deltaSnapshotSizeThresholdPercentage float64,
	downloadTargets []*DownloadTarget,
	solidEntryPointCheckThresholdPast milestone.Index,
	solidEntryPointCheckThresholdFuture milestone.Index,
	additionalPruningThreshold milestone.Index,
	snapshotDepth milestone.Index,
	snapshotInterval milestone.Index,
	pruningMilestonesEnabled bool,
	pruningMilestonesMaxMilestonesToKeep milestone.Index,
	pruningSizeEnabled bool,
	pruningSizeTargetSizeBytes int64,
	pruningSizeThresholdPercentage float64,
	pruningSizeCooldownTime time.Duration,
	pruneReceipts bool) *SnapshotManager {

	return &SnapshotManager{
		shutdownCtx:                          shutdownCtx,
		log:                                  log,
		database:                             database,
		storage:                              storage,
		syncManager:                          syncManager,
		utxoManager:                          utxoManager,
		networkID:                            networkID,
		networkIDSource:                      networkIDSource,
		snapshotFullPath:                     snapshotFullPath,
		snapshotDeltaPath:                    snapshotDeltaPath,
		deltaSnapshotSizeThresholdPercentage: deltaSnapshotSizeThresholdPercentage,
		downloadTargets:                      downloadTargets,
		solidEntryPointCheckThresholdPast:    solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture:  solidEntryPointCheckThresholdFuture,
		additionalPruningThreshold:           additionalPruningThreshold,
		snapshotDepth:                        snapshotDepth,
		snapshotInterval:                     snapshotInterval,
		pruningMilestonesEnabled:             pruningMilestonesEnabled,
		pruningMilestonesMaxMilestonesToKeep: pruningMilestonesMaxMilestonesToKeep,
		pruningSizeEnabled:                   pruningSizeEnabled,
		pruningSizeTargetSizeBytes:           pruningSizeTargetSizeBytes,
		pruningSizeThresholdPercentage:       pruningSizeThresholdPercentage,
		pruningSizeCooldownTime:              pruningSizeCooldownTime,
		pruneReceipts:                        pruneReceipts,
		Events: &Events{
			SnapshotMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
			SnapshotMetricsUpdated:        events.NewEvent(SnapshotMetricsCaller),
			PruningMilestoneIndexChanged:  events.NewEvent(milestone.IndexCaller),
			PruningMetricsUpdated:         events.NewEvent(PruningMetricsCaller),
		},
	}
}

func (s *SnapshotManager) IsSnapshottingOrPruning() bool {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.isSnapshotting || s.isPruning
}

func (s *SnapshotManager) shouldTakeSnapshot(confirmedMilestoneIndex milestone.Index) bool {

	snapshotInfo := s.storage.SnapshotInfo()
	if snapshotInfo == nil {
		s.log.Panic("No snapshotInfo found!")
	}

	if (confirmedMilestoneIndex < s.snapshotDepth+s.snapshotInterval) || (confirmedMilestoneIndex-s.snapshotDepth) < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		// Not enough history to calculate solid entry points
		return false
	}

	return confirmedMilestoneIndex-(s.snapshotDepth+s.snapshotInterval) >= snapshotInfo.SnapshotIndex
}

func (s *SnapshotManager) forEachSolidEntryPoint(targetIndex milestone.Index, abortSignal <-chan struct{}, solidEntryPointConsumer func(sep *storage.SolidEntryPoint) bool) error {

	solidEntryPoints := make(map[string]milestone.Index)

	metadataMemcache := storage.NewMetadataMemcache(s.storage)
	defer metadataMemcache.Cleanup(true)

	// we share the same traverser for all milestones, so we don't cleanup the cachedMessages in between.
	// we don't need to call cleanup at the end, because we passed our own metadataMemcache.
	parentsTraverser := dag.NewParentTraverser(s.storage, metadataMemcache)

	// isSolidEntryPoint checks whether any direct child of the given message was referenced by a milestone which is above the target milestone.
	isSolidEntryPoint := func(messageID hornet.MessageID, targetIndex milestone.Index) bool {
		for _, childMessageID := range s.storage.ChildrenMessageIDs(messageID) {
			cachedMsgMeta := metadataMemcache.CachedMetadataOrNil(childMessageID) // meta +1
			if cachedMsgMeta == nil {
				// Ignore this message since it doesn't exist anymore
				s.log.Warnf("%s, msg ID: %v, child msg ID: %v", ErrChildMsgNotFound, messageID.ToHex(), childMessageID.ToHex())
				continue
			}

			if referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex(); referenced && (at > targetIndex) {
				// referenced by a later milestone than targetIndex => solidEntryPoint
				return true
			}
		}
		return false
	}

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - s.solidEntryPointCheckThresholdPast; milestoneIndex <= targetIndex; milestoneIndex++ {
		select {
		case <-abortSignal:
			return ErrSnapshotCreationWasAborted
		default:
		}

		cachedMilestone := s.storage.CachedMilestoneOrNil(milestoneIndex) // milestone +1
		if cachedMilestone == nil {
			return errors.Wrapf(ErrCritical, "milestone (%d) not found!", milestoneIndex)
		}

		// Get all parents of that milestone
		milestoneMessageID := cachedMilestone.Milestone().MessageID
		cachedMilestone.Release(true) // message -1

		// traverse the milestone and collect all messages that were referenced by this milestone or newer
		if err := parentsTraverser.Traverse(hornet.MessageIDs{milestoneMessageID},
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // msg +1
				defer cachedMsgMeta.Release(true) // msg -1

				// collect all msg that were referenced by that milestone or newer
				referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex()
				return referenced && at >= milestoneIndex, nil
			},
			// consumer
			func(cachedMsgMeta *storage.CachedMetadata) error { // msg +1
				defer cachedMsgMeta.Release(true) // msg -1

				select {
				case <-abortSignal:
					return ErrSnapshotCreationWasAborted
				default:
				}

				messageID := cachedMsgMeta.Metadata().MessageID()

				if isEntryPoint := isSolidEntryPoint(messageID, targetIndex); !isEntryPoint {
					return nil
				}

				referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex()
				if !referenced {
					return errors.Wrapf(ErrCritical, "solid entry point (%v) not referenced!", messageID.ToHex())
				}

				messageIDMapKey := messageID.ToMapKey()
				if _, exists := solidEntryPoints[messageIDMapKey]; !exists {
					solidEntryPoints[messageIDMapKey] = at
					if !solidEntryPointConsumer(&storage.SolidEntryPoint{MessageID: messageID, Index: at}) {
						return ErrSnapshotCreationWasAborted
					}
				}

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
			if errors.Is(err, common.ErrOperationAborted) {
				return ErrSnapshotCreationWasAborted
			}
		}
	}

	return nil
}

func (s *SnapshotManager) checkSnapshotLimits(targetIndex milestone.Index, snapshotInfo *storage.SnapshotInfo, writeToDatabase bool) error {

	confirmedMilestoneIndex := s.syncManager.ConfirmedMilestoneIndex()

	if confirmedMilestoneIndex < s.solidEntryPointCheckThresholdFuture {
		return errors.Wrapf(ErrNotEnoughHistory, "minimum confirmed index: %d, actual confirmed index: %d", s.solidEntryPointCheckThresholdFuture+1, confirmedMilestoneIndex)
	}

	minimumIndex := s.solidEntryPointCheckThresholdPast + 1
	maximumIndex := confirmedMilestoneIndex - s.solidEntryPointCheckThresholdFuture

	if writeToDatabase && minimumIndex < snapshotInfo.SnapshotIndex+1 {
		// if we write the snapshot state to the database, the newly generated snapshot index must be greater than the last snapshot index
		minimumIndex = snapshotInfo.SnapshotIndex + 1
	}

	if minimumIndex < snapshotInfo.PruningIndex+1+s.solidEntryPointCheckThresholdPast {
		// since we always generate new solid entry points, we need enough history
		minimumIndex = snapshotInfo.PruningIndex + 1 + s.solidEntryPointCheckThresholdPast
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

func (s *SnapshotManager) setIsSnapshotting(value bool) {
	s.statusLock.Lock()
	s.isSnapshotting = value
	s.statusLock.Unlock()
}

// CreateFullSnapshot creates a full snapshot for the given target milestone index.
func (s *SnapshotManager) CreateFullSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createSnapshotWithoutLocking(Full, targetIndex, filePath, writeToDatabase, abortSignal)
}

// CreateDeltaSnapshot creates a delta snapshot for the given target milestone index.
func (s *SnapshotManager) CreateDeltaSnapshot(targetIndex milestone.Index, filePath string, writeToDatabase bool, abortSignal <-chan struct{}, snapshotFullPath ...string) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()
	return s.createSnapshotWithoutLocking(Delta, targetIndex, filePath, writeToDatabase, abortSignal, snapshotFullPath...)
}

// LoadSnapshotFromFile loads a snapshot file from the given file path into the storage.
func (s *SnapshotManager) LoadSnapshotFromFile(snapshotType Type, networkID uint64, filePath string) (err error) {
	s.log.Infof("importing %s snapshot file...", snapshotNames[snapshotType])
	ts := time.Now()

	header, err := loadSnapshotFileToStorage(s.shutdownCtx, s.storage, snapshotType, filePath, networkID)
	if err != nil {
		return err
	}

	if err := s.syncManager.SetConfirmedMilestoneIndex(header.SEPMilestoneIndex, false); err != nil {
		return fmt.Errorf("SetConfirmedMilestoneIndex failed: %w", err)
	}

	s.log.Infof("imported %s snapshot file, took %v", snapshotNames[snapshotType], time.Since(ts).Truncate(time.Millisecond))
	s.log.Infof("solid entry points: %d, outputs: %d, ms diffs: %d", header.SEPCount, header.OutputCount, header.MilestoneDiffCount)
	s.log.Infof(`
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
func (s *SnapshotManager) optimalSnapshotType() (Type, error) {
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
func (s *SnapshotManager) snapshotTypeFilePath(snapshotType Type) string {
	switch snapshotType {
	case Full:
		return s.snapshotFullPath
	case Delta:
		return s.snapshotDeltaPath
	default:
		panic("unknown snapshot type")
	}
}

// HandleNewConfirmedMilestoneEvent handles new confirmed milestone events which may trigger a delta snapshot creation and pruning.
func (s *SnapshotManager) HandleNewConfirmedMilestoneEvent(confirmedMilestoneIndex milestone.Index, shutdownSignal <-chan struct{}) {
	if !s.syncManager.IsNodeSynced() {
		// do not prune or create snapshots while we are not synced
		return
	}

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	if s.shouldTakeSnapshot(confirmedMilestoneIndex) {
		snapshotType, err := s.optimalSnapshotType()
		if err != nil {
			s.log.Warnf("%s: %s", ErrSnapshotCreationFailed, err)
			return
		}

		if err := s.createSnapshotWithoutLocking(snapshotType, confirmedMilestoneIndex-s.snapshotDepth, s.snapshotTypeFilePath(snapshotType), true, shutdownSignal); err != nil {
			if errors.Is(err, ErrCritical) {
				s.log.Panicf("%s: %s", ErrSnapshotCreationFailed, err)
			}
			s.log.Warnf("%s: %s", ErrSnapshotCreationFailed, err)
		}

		if !s.syncManager.IsNodeSynced() {
			// do not prune while we are not synced
			return
		}
	}

	var targetIndex milestone.Index = 0
	if s.pruningMilestonesEnabled && confirmedMilestoneIndex > s.pruningMilestonesMaxMilestonesToKeep {
		targetIndex = confirmedMilestoneIndex - s.pruningMilestonesMaxMilestonesToKeep
	}

	pruningBySize := false
	if s.pruningSizeEnabled && (s.lastPruningBySizeTime.IsZero() || time.Since(s.lastPruningBySizeTime) > s.pruningSizeCooldownTime) {
		targetIndexSize, err := s.calcTargetIndexBySize()
		if err == nil && ((targetIndex == 0) || (targetIndex < targetIndexSize)) {
			targetIndex = targetIndexSize
			pruningBySize = true
		}
	}

	if targetIndex == 0 {
		// no pruning needed
		return
	}

	if _, err := s.pruneDatabase(targetIndex, shutdownSignal); err != nil {
		s.log.Debugf("pruning aborted: %v", err)
	}

	if pruningBySize {
		s.lastPruningBySizeTime = time.Now()
	}
}

// SnapshotsFilesLedgerIndex returns the final ledger index if the snapshots from the configured file paths would be applied.
func (s *SnapshotManager) SnapshotsFilesLedgerIndex() (milestone.Index, error) {

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
func (s *SnapshotManager) ImportSnapshots() error {
	snapAvail, err := s.checkSnapshotFilesAvailability(s.snapshotFullPath, s.snapshotDeltaPath)
	if err != nil {
		return err
	}

	if snapAvail == snapshotAvailNone {
		if err = s.downloadSnapshotFiles(s.networkID, s.snapshotFullPath, s.snapshotDeltaPath); err != nil {
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

	if err = s.LoadSnapshotFromFile(Full, s.networkID, s.snapshotFullPath); err != nil {
		_ = s.storage.MarkDatabaseCorrupted()
		return err
	}

	if snapAvail == snapshotAvailOnlyFull {
		return nil
	}

	if err = s.LoadSnapshotFromFile(Delta, s.networkID, s.snapshotDeltaPath); err != nil {
		_ = s.storage.MarkDatabaseCorrupted()
		return err
	}

	return nil
}

// checks that either both snapshot files are available, only the full snapshot or none.
func (s *SnapshotManager) checkSnapshotFilesAvailability(fullPath string, deltaPath string) (snapshotAvailability, error) {
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
func (s *SnapshotManager) downloadSnapshotFiles(wantedNetworkID uint64, fullPath string, deltaPath string) error {
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
	s.log.Infof("downloading snapshot files from one of the provided sources %s", string(targetsJSON))

	if err := s.DownloadSnapshotFiles(wantedNetworkID, fullPath, deltaPath, s.downloadTargets); err != nil {
		return fmt.Errorf("unable to download snapshot files: %w", err)
	}

	s.log.Info("snapshot download finished")
	return nil
}

// CheckCurrentSnapshot checks that the current snapshot info is valid regarding its network ID and the ledger state.
func (s *SnapshotManager) CheckCurrentSnapshot(snapshotInfo *storage.SnapshotInfo) error {

	// check that the stored snapshot corresponds to the wanted network ID
	if snapshotInfo.NetworkID != s.networkID {
		s.log.Panicf("node is configured to operate in network %d/%s but the stored snapshot data corresponds to %d", s.networkID, s.networkIDSource, snapshotInfo.NetworkID)
	}

	// if we don't enforce loading of a snapshot,
	// we can check the ledger state of the current database and start the node.
	if err := s.utxoManager.CheckLedgerState(); err != nil {
		s.log.Fatal(err)
	}

	return nil
}
