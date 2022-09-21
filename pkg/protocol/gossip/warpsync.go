package gossip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	iotago "github.com/iotaledger/iota.go/v3"
)

// NewWarpSync creates a new WarpSync instance with the given advancement range and criteria func.
// If no advancement func is provided, the WarpSync uses AdvanceAtPercentageReached with DefaultAdvancementThreshold.
func NewWarpSync(advRange int, advanceCheckpointCriteriaFunc ...AdvanceCheckpointCriteria) *WarpSync {
	ws := &WarpSync{
		AdvancementRange: syncmanager.MilestoneIndexDelta(advRange),
		Events: &Events{
			CheckpointUpdated: events.NewEvent(CheckpointCaller),
			TargetUpdated:     events.NewEvent(TargetCaller),
			Start:             events.NewEvent(SyncStartCaller),
			Done:              events.NewEvent(SyncDoneCaller),
		},
	}
	if len(advanceCheckpointCriteriaFunc) > 0 {
		ws.advCheckpointCriteria = advanceCheckpointCriteriaFunc[0]
	} else {
		ws.advCheckpointCriteria = AdvanceAtPercentageReached(DefaultAdvancementThreshold)
	}

	return ws
}

func SyncStartCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(target iotago.MilestoneIndex, newCheckpoint iotago.MilestoneIndex, msRange syncmanager.MilestoneIndexDelta))(params[0].(iotago.MilestoneIndex), params[1].(iotago.MilestoneIndex), params[2].(syncmanager.MilestoneIndexDelta))
}

func SyncDoneCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(delta int, referencedBlocksTotal int, dur time.Duration))(params[0].(int), params[1].(int), params[2].(time.Duration))
}

func CheckpointCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(newCheckpoint iotago.MilestoneIndex, oldCheckpoint iotago.MilestoneIndex, msRange syncmanager.MilestoneIndexDelta, target iotago.MilestoneIndex))(params[0].(iotago.MilestoneIndex), params[1].(iotago.MilestoneIndex), params[2].(syncmanager.MilestoneIndexDelta), params[3].(iotago.MilestoneIndex))
}

func TargetCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(checkpoint iotago.MilestoneIndex, target iotago.MilestoneIndex))(params[0].(iotago.MilestoneIndex), params[1].(iotago.MilestoneIndex))
}

// Events holds WarpSync related events.
type Events struct {
	// Fired when a new set of milestones should be requested.
	CheckpointUpdated *events.Event
	// Fired when the target milestone is updated.
	TargetUpdated *events.Event
	// Fired when warp synchronization starts.
	Start *events.Event
	// Fired when the warp synchronization is done.
	Done *events.Event
}

// AdvanceCheckpointCriteria is a function which determines whether the checkpoint should be advanced.
type AdvanceCheckpointCriteria func(currentConfirmed, previousCheckpoint, currentCheckpoint iotago.MilestoneIndex) bool

// DefaultAdvancementThreshold is the default threshold at which a checkpoint advancement is done.
// Per default an advancement is always done as soon the confirmed milestone enters the range between
// the previous and current checkpoint.
const DefaultAdvancementThreshold = 0.0

// AdvanceAtPercentageReached is an AdvanceCheckpointCriteria which advances the checkpoint
// when the current one was reached by >=X% by the current confirmed milestone in relation to the previous checkpoint.
func AdvanceAtPercentageReached(threshold float64) AdvanceCheckpointCriteria {
	return func(currentConfirmed, previousCheckpoint, currentCheckpoint iotago.MilestoneIndex) bool {
		// the previous checkpoint can be over the current confirmed milestone,
		// as advancements move the checkpoint window above the confirmed milestone
		if currentConfirmed < previousCheckpoint {
			return false
		}
		checkpointDelta := currentCheckpoint - previousCheckpoint
		progress := currentConfirmed - previousCheckpoint

		return float64(progress)/float64(checkpointDelta) >= threshold
	}
}

// WarpSync is metadata about doing a synchronization via STING messages.
type WarpSync struct {
	sync.Mutex

	// The used advancement range per checkpoint.
	AdvancementRange syncmanager.MilestoneIndexDelta
	// The Events of the warpsync.
	Events *Events
	// The criteria whether to advance to the next checkpoint.
	advCheckpointCriteria AdvanceCheckpointCriteria

	// The current confirmed milestone of the node.
	CurrentConfirmedMilestone iotago.MilestoneIndex
	// The starting time of the synchronization.
	StartTime time.Time
	// The starting point of the synchronization.
	InitMilestone iotago.MilestoneIndex
	// The target milestone to which to synchronize to.
	TargetMilestone iotago.MilestoneIndex
	// The previous checkpoint of the synchronization.
	PreviousCheckpoint iotago.MilestoneIndex
	// The current checkpoint of the synchronization.
	CurrentCheckpoint iotago.MilestoneIndex
	// The amount of referenced blocks during this warpsync run.
	referencedBlocksTotal int
}

// UpdateCurrentConfirmedMilestone updates the current confirmed milestone index state.
func (ws *WarpSync) UpdateCurrentConfirmedMilestone(current iotago.MilestoneIndex) {
	ws.Lock()
	defer ws.Unlock()

	if current <= ws.CurrentConfirmedMilestone {
		return
	}
	ws.CurrentConfirmedMilestone = current

	// synchronization not started
	if ws.CurrentCheckpoint == 0 {
		return
	}

	// finished
	if ws.TargetMilestone != 0 && ws.CurrentConfirmedMilestone >= ws.TargetMilestone {
		ws.Events.Done.Trigger(int(ws.TargetMilestone-ws.InitMilestone), ws.referencedBlocksTotal, time.Since(ws.StartTime))
		ws.reset()

		return
	}

	// check whether advancement criteria is fulfilled
	if !ws.advCheckpointCriteria(ws.CurrentConfirmedMilestone, ws.PreviousCheckpoint, ws.CurrentCheckpoint) {
		return
	}

	oldCheckpoint := ws.CurrentCheckpoint
	if msRange := ws.advanceCheckpoint(); msRange != 0 {
		ws.Events.CheckpointUpdated.Trigger(ws.CurrentCheckpoint, oldCheckpoint, msRange, ws.TargetMilestone)
	}
}

// UpdateTargetMilestone updates the synchronization target if it is higher than the current one and
// triggers a synchronization start if the target was set for the first time.
func (ws *WarpSync) UpdateTargetMilestone(target iotago.MilestoneIndex) {
	ws.Lock()
	defer ws.Unlock()

	if target <= ws.TargetMilestone {
		return
	}

	ws.TargetMilestone = target

	// as a special case, while we are warp syncing and within the last checkpoint range,
	// new target milestones need to shift the checkpoint to the new target, in order
	// to fire an 'updated checkpoint event'/respectively updating the request queue filter.
	// since we will request missing parents for the new target, it will still solidify
	// even though we discarded requests for a short period of time parents when the
	// request filter wasn't yet updated.
	if ws.CurrentCheckpoint != 0 && ws.CurrentCheckpoint+ws.AdvancementRange > ws.TargetMilestone {
		oldCheckpoint := ws.CurrentCheckpoint
		reqRange := ws.TargetMilestone - ws.CurrentCheckpoint
		ws.CurrentCheckpoint = ws.TargetMilestone
		ws.Events.CheckpointUpdated.Trigger(ws.CurrentCheckpoint, oldCheckpoint, reqRange, ws.TargetMilestone)
	}

	if ws.CurrentCheckpoint != 0 {
		// if synchronization was already started, only update the target
		ws.Events.TargetUpdated.Trigger(ws.CurrentCheckpoint, ws.TargetMilestone)

		return
	}

	// do not start the synchronization if current confirmed is newer than the target or the delta is smaller than 2
	if ws.CurrentConfirmedMilestone >= ws.TargetMilestone || target-ws.CurrentConfirmedMilestone < 2 {
		return
	}

	// start the synchronization
	ws.StartTime = time.Now()
	ws.InitMilestone = ws.CurrentConfirmedMilestone
	ws.PreviousCheckpoint = ws.CurrentConfirmedMilestone
	advancementRange := ws.advanceCheckpoint()
	ws.Events.Start.Trigger(ws.TargetMilestone, ws.CurrentCheckpoint, advancementRange)
}

// AddReferencedBlocksCount adds the amount of referenced blocks to collect stats.
func (ws *WarpSync) AddReferencedBlocksCount(referencedBlocksCount int) {
	ws.Lock()
	defer ws.Unlock()

	ws.referencedBlocksTotal += referencedBlocksCount
}

// advances the next checkpoint by either incrementing from the current
// via the checkpoint range or max to the target of the synchronization.
// returns the chosen range.
func (ws *WarpSync) advanceCheckpoint() syncmanager.MilestoneIndexDelta {
	if ws.CurrentCheckpoint != 0 {
		ws.PreviousCheckpoint = ws.CurrentCheckpoint
	}

	advRange := ws.AdvancementRange

	// make sure we advance max to the target milestone
	if ws.TargetMilestone-ws.CurrentConfirmedMilestone <= advRange || ws.TargetMilestone-ws.CurrentCheckpoint <= advRange {
		deltaRange := ws.TargetMilestone - ws.CurrentCheckpoint
		if deltaRange > ws.TargetMilestone-ws.CurrentConfirmedMilestone {
			deltaRange = ws.TargetMilestone - ws.CurrentConfirmedMilestone
		}
		ws.CurrentCheckpoint = ws.TargetMilestone

		return deltaRange
	}

	// at start simply advance from the current confirmed
	if ws.CurrentCheckpoint == 0 {
		ws.CurrentCheckpoint = ws.CurrentConfirmedMilestone + advRange

		return advRange
	}

	ws.CurrentCheckpoint = ws.CurrentCheckpoint + advRange

	return advRange
}

// resets the warp sync.
func (ws *WarpSync) reset() {
	ws.StartTime = time.Time{}
	ws.InitMilestone = 0
	ws.TargetMilestone = 0
	ws.PreviousCheckpoint = 0
	ws.CurrentCheckpoint = 0
	ws.referencedBlocksTotal = 0
}

// WarpSyncMilestoneRequester walks the cones of existing but non-solid milestones and memoizes already walked blocks and milestones.
type WarpSyncMilestoneRequester struct {
	syncutils.Mutex

	// used to cancel the warp sync requester.
	ctx context.Context
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// used to request blocks from peers.
	requester *Requester
	// do not remove requests if the enqueue time is over the given threshold.
	preventDiscard bool
	// map of already traversed blocks to to prevent traversing the same cones multiple times.
	traversed map[iotago.BlockID]struct{}
}

// NewWarpSyncMilestoneRequester creates a new WarpSyncMilestoneRequester instance.
func NewWarpSyncMilestoneRequester(
	ctx context.Context,
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	requester *Requester,
	preventDiscard bool) *WarpSyncMilestoneRequester {

	return &WarpSyncMilestoneRequester{
		ctx:            ctx,
		storage:        dbStorage,
		syncManager:    syncManager,
		requester:      requester,
		preventDiscard: preventDiscard,
		traversed:      make(map[iotago.BlockID]struct{}),
	}
}

// requestMissingMilestoneParents traverses the parents of a given milestone and requests each missing parent.
// Already requested milestones or traversed blocks will be ignored, to circumvent requesting
// the same parents multiple times.
func (w *WarpSyncMilestoneRequester) requestMissingMilestoneParents(msIndex iotago.MilestoneIndex) error {
	if msIndex <= w.syncManager.ConfirmedMilestoneIndex() {
		return nil
	}

	// get all parents of that milestone
	milestoneParents, err := w.storage.MilestoneParentsByIndex(msIndex)
	if err != nil {
		return fmt.Errorf("milestone doesn't exist (%d)", msIndex)
	}

	return dag.TraverseParents(
		w.ctx,
		w.storage,
		milestoneParents,
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			blockID := cachedBlockMeta.Metadata().BlockID()
			if _, previouslyTraversed := w.traversed[blockID]; previouslyTraversed {
				return false, nil
			}
			w.traversed[blockID] = struct{}{}

			if cachedBlockMeta.Metadata().IsSolid() {
				return false, nil
			}

			return true, nil
		},
		// consumer
		nil,
		// called on missing parents
		func(parentBlockID iotago.BlockID) error {
			w.requester.Request(parentBlockID, msIndex, w.preventDiscard)

			return nil
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false)
}

// Cleanup cleans up traversed blocks to free memory.
func (w *WarpSyncMilestoneRequester) Cleanup() {
	w.Lock()
	defer w.Unlock()

	w.traversed = make(map[iotago.BlockID]struct{})
}

// RequestMilestoneRange requests up to N milestones nearest to the current confirmed milestone index.
// Returns the number of milestones requested.
func (w *WarpSyncMilestoneRequester) RequestMilestoneRange(rangeToRequest syncmanager.MilestoneIndexDelta, from ...iotago.MilestoneIndex) (syncmanager.MilestoneIndexDelta, iotago.MilestoneIndex, iotago.MilestoneIndex) {
	w.Lock()
	defer w.Unlock()

	var requested syncmanager.MilestoneIndexDelta

	startingPoint := w.syncManager.ConfirmedMilestoneIndex()
	if len(from) > 0 {
		startingPoint = from[0]
	}

	startIndex := startingPoint + 1
	endIndex := startingPoint + rangeToRequest

	var msIndexes []iotago.MilestoneIndex
	for i := syncmanager.MilestoneIndexDelta(1); i <= rangeToRequest; i++ {
		msIndexToRequest := startingPoint + i

		if !w.storage.ContainsMilestoneIndex(msIndexToRequest) {
			// only request if we don't have the milestone
			requested++
			msIndexes = append(msIndexes, msIndexToRequest)

			continue
		}

		// milestone already exists
		if err := w.requestMissingMilestoneParents(msIndexToRequest); err != nil && errors.Is(err, common.ErrOperationAborted) {
			// do not proceed if the node was shut down
			return 0, 0, 0
		}
	}

	// enqueue every milestone request to the request queue
	for _, msIndex := range msIndexes {
		w.requester.Request(msIndex, msIndex)
	}

	return requested, startIndex, endIndex
}
