package gossip

import (
	"fmt"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
)

// NewWarpSync creates a new WarpSync instance with the given advancement range and criteria func.
// If no advancement func is provided, the WarpSync uses AdvanceAtPercentageReached with DefaultAdvancementThreshold.
func NewWarpSync(advRange int, advanceCheckpointCriteriaFunc ...AdvanceCheckpointCriteria) *WarpSync {
	ws := &WarpSync{
		AdvancementRange: advRange,
		Events: Events{
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
	handler.(func(target milestone.Index, newCheckpoint milestone.Index, msRange int32))(params[0].(milestone.Index), params[1].(milestone.Index), params[2].(int32))
}

func SyncDoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(delta int, referencedMessagesTotal int, dur time.Duration))(params[0].(int), params[1].(int), params[2].(time.Duration))
}

func CheckpointCaller(handler interface{}, params ...interface{}) {
	handler.(func(newCheckpoint milestone.Index, oldCheckpoint milestone.Index, msRange int32, target milestone.Index))(params[0].(milestone.Index), params[1].(milestone.Index), params[2].(int32), params[3].(milestone.Index))
}

func TargetCaller(handler interface{}, params ...interface{}) {
	handler.(func(checkpoint milestone.Index, target milestone.Index))(params[0].(milestone.Index), params[1].(milestone.Index))
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
type AdvanceCheckpointCriteria func(currentConfirmed, previousCheckpoint, currentCheckpoint milestone.Index) bool

// DefaultAdvancementThreshold is the default threshold at which a checkpoint advancement is done.
// Per default an advancement is always done as soon the confirmed milestone enters the range between
// the previous and current checkpoint.
const DefaultAdvancementThreshold = 0.0

// AdvanceAtPercentageReached is an AdvanceCheckpointCriteria which advances the checkpoint
// when the current one was reached by >=X% by the current confirmed milestone in relation to the previous checkpoint.
func AdvanceAtPercentageReached(threshold float64) AdvanceCheckpointCriteria {
	return func(currentConfirmed, previousCheckpoint, currentCheckpoint milestone.Index) bool {
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
	AdvancementRange int
	// The Events of the warpsync.
	Events Events
	// The criteria whether to advance to the next checkpoint.
	advCheckpointCriteria AdvanceCheckpointCriteria

	// The current confirmed milestone of the node.
	CurrentConfirmedMilestone milestone.Index
	// The starting time of the synchronization.
	StartTime time.Time
	// The starting point of the synchronization.
	InitMilestone milestone.Index
	// The target milestone to which to synchronize to.
	TargetMilestone milestone.Index
	// The previous checkpoint of the synchronization.
	PreviousCheckpoint milestone.Index
	// The current checkpoint of the synchronization.
	CurrentCheckpoint milestone.Index
	// The amount of referenced messages during this warpsync run.
	referencedMessagesTotal int
}

// UpdateCurrentConfirmedMilestone updates the current confirmed milestone index state.
func (ws *WarpSync) UpdateCurrentConfirmedMilestone(current milestone.Index) {
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
		ws.Events.Done.Trigger(int(ws.TargetMilestone-ws.InitMilestone), ws.referencedMessagesTotal, time.Since(ws.StartTime))
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
func (ws *WarpSync) UpdateTargetMilestone(target milestone.Index) {
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
	if ws.CurrentCheckpoint != 0 && ws.CurrentCheckpoint+milestone.Index(ws.AdvancementRange) > ws.TargetMilestone {
		oldCheckpoint := ws.CurrentCheckpoint
		reqRange := ws.TargetMilestone - ws.CurrentCheckpoint
		ws.CurrentCheckpoint = ws.TargetMilestone
		ws.Events.CheckpointUpdated.Trigger(ws.CurrentCheckpoint, oldCheckpoint, int32(reqRange), ws.TargetMilestone)
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

// AddReferencedMessagesCount adds the amount of referenced messages to collect stats.
func (ws *WarpSync) AddReferencedMessagesCount(referencedMessagesCount int) {
	ws.Lock()
	defer ws.Unlock()

	ws.referencedMessagesTotal += referencedMessagesCount
}

// advances the next checkpoint by either incrementing from the current
// via the checkpoint range or max to the target of the synchronization.
// returns the chosen range.
func (ws *WarpSync) advanceCheckpoint() int32 {
	if ws.CurrentCheckpoint != 0 {
		ws.PreviousCheckpoint = ws.CurrentCheckpoint
	}

	advRange := milestone.Index(ws.AdvancementRange)

	// make sure we advance max to the target milestone
	if ws.TargetMilestone-ws.CurrentConfirmedMilestone <= advRange || ws.TargetMilestone-ws.CurrentCheckpoint <= advRange {
		deltaRange := ws.TargetMilestone - ws.CurrentCheckpoint
		if deltaRange > ws.TargetMilestone-ws.CurrentConfirmedMilestone {
			deltaRange = ws.TargetMilestone - ws.CurrentConfirmedMilestone
		}
		ws.CurrentCheckpoint = ws.TargetMilestone
		return int32(deltaRange)
	}

	// at start simply advance from the current confirmed
	if ws.CurrentCheckpoint == 0 {
		ws.CurrentCheckpoint = ws.CurrentConfirmedMilestone + advRange
		return int32(advRange)
	}

	ws.CurrentCheckpoint = ws.CurrentCheckpoint + advRange
	return int32(advRange)
}

// resets the warp sync.
func (ws *WarpSync) reset() {
	ws.StartTime = time.Time{}
	ws.InitMilestone = 0
	ws.TargetMilestone = 0
	ws.PreviousCheckpoint = 0
	ws.CurrentCheckpoint = 0
	ws.referencedMessagesTotal = 0
}

// NewWarpSyncMilestoneRequester creates a new WarpSyncMilestoneRequester instance.
func NewWarpSyncMilestoneRequester(storage *storage.Storage, requester *Requester, preventDiscard bool) *WarpSyncMilestoneRequester {
	return &WarpSyncMilestoneRequester{
		storage:        storage,
		requester:      requester,
		preventDiscard: preventDiscard,
		traversed:      make(map[string]struct{}),
	}
}

// WarpSyncMilestoneRequester walks the cones of existing but non-solid milestones and memoizes already walked messages and milestones.
type WarpSyncMilestoneRequester struct {
	syncutils.Mutex

	storage        *storage.Storage
	requester      *Requester
	preventDiscard bool
	traversed      map[string]struct{}
}

// RequestMissingMilestoneParents traverses the parents of a given milestone and requests each missing parent.
// Already requested milestones or traversed messages will be ignored, to circumvent requesting
// the same parents multiple times.
func (w *WarpSyncMilestoneRequester) RequestMissingMilestoneParents(msIndex milestone.Index) {
	w.Lock()
	defer w.Unlock()

	if msIndex <= w.storage.ConfirmedMilestoneIndex() {
		return
	}

	cachedMs := w.storage.CachedMilestoneOrNil(msIndex) // milestone +1
	if cachedMs == nil {
		panic(fmt.Sprintf("milestone %d wasn't found", msIndex))
	}

	milestoneMessageID := cachedMs.Milestone().MessageID
	cachedMs.Release(true) // message -1

	// error is ignored because the next milestone will repeat the process anyway
	_ = dag.TraverseParentsOfMessage(w.storage, milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			mapKey := cachedMsgMeta.Metadata().MessageID().ToMapKey()
			if _, previouslyTraversed := w.traversed[mapKey]; previouslyTraversed {
				return false, nil
			}
			w.traversed[mapKey] = struct{}{}

			if cachedMsgMeta.Metadata().IsSolid() {
				return false, nil
			}

			return true, nil
		},
		// consumer
		nil,
		// called on missing parents
		func(parentMessageID hornet.MessageID) error {
			w.requester.Request(parentMessageID, msIndex, w.preventDiscard)
			return nil
		},
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false, nil)
}

// Cleanup cleans up traversed messages to free memory.
func (w *WarpSyncMilestoneRequester) Cleanup() {
	w.Lock()
	defer w.Unlock()

	w.traversed = make(map[string]struct{})
}
