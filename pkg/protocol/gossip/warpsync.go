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
		Events: Events{
			CheckpointUpdated: events.NewEvent(CheckpointCaller),
			TargetUpdated:     events.NewEvent(TargetCaller),
			Start:             events.NewEvent(SyncStartCaller),
			Done:              events.NewEvent(SyncDoneCaller),
		},
		AdvancementRange: advRange,
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
	mu                    sync.Mutex
	start                 time.Time
	advCheckpointCriteria AdvanceCheckpointCriteria
	Events                Events
	// The starting point of the synchronization.
	Init milestone.Index
	// The current confirmed milestone of the node.
	CurrentConfirmedMs milestone.Index
	// The target milestone to which to synchronize to.
	TargetMs milestone.Index
	// The previous checkpoint of the synchronization.
	PreviousCheckpoint milestone.Index
	// The current checkpoint of the synchronization.
	CurrentCheckpoint milestone.Index
	// The used advancement range per checkpoint.
	AdvancementRange int
	// The amount of referenced messages during this warpsync run.
	referencedMessagesTotal int
}

// UpdateCurrent updates the current confirmed milestone index state.
func (ws *WarpSync) UpdateCurrent(current milestone.Index) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if current <= ws.CurrentConfirmedMs {
		return
	}
	ws.CurrentConfirmedMs = current

	// synchronization not started
	if ws.CurrentCheckpoint == 0 {
		return
	}

	// finished
	if ws.TargetMs != 0 && ws.CurrentConfirmedMs >= ws.TargetMs {
		ws.Events.Done.Trigger(int(ws.TargetMs-ws.Init), ws.referencedMessagesTotal, time.Since(ws.start))
		ws.reset()
		return
	}

	// check whether advancement criteria is fulfilled
	if !ws.advCheckpointCriteria(ws.CurrentConfirmedMs, ws.PreviousCheckpoint, ws.CurrentCheckpoint) {
		return
	}

	oldCheckpoint := ws.CurrentCheckpoint
	if msRange := ws.advanceCheckpoint(); msRange != 0 {
		ws.Events.CheckpointUpdated.Trigger(ws.CurrentCheckpoint, oldCheckpoint, msRange, ws.TargetMs)
	}
}

// UpdateTarget updates the synchronization target if it is higher than the current one and
// triggers a synchronization start if the target was set for the first time.
func (ws *WarpSync) UpdateTarget(target milestone.Index) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if target <= ws.TargetMs {
		return
	}

	ws.TargetMs = target

	// as a special case, while we are warp syncing and within the last checkpoint range,
	// new target milestones need to shift the checkpoint to the new target, in order
	// to fire an 'updated checkpoint event'/respectively updating the request queue filter.
	// since we will request missing parents for the new target, it will still solidify
	// even though we discarded requests for a short period of time parents when the
	// request filter wasn't yet updated.
	if ws.TargetMs != 0 && ws.CurrentCheckpoint+milestone.Index(ws.AdvancementRange) > ws.TargetMs {
		oldCheckpoint := ws.CurrentCheckpoint
		reqRange := ws.TargetMs - ws.CurrentCheckpoint
		ws.CurrentCheckpoint = ws.TargetMs
		ws.Events.CheckpointUpdated.Trigger(ws.CurrentCheckpoint, oldCheckpoint, int32(reqRange), ws.TargetMs)
	}

	if ws.CurrentCheckpoint != 0 {
		ws.Events.TargetUpdated.Trigger(ws.CurrentCheckpoint, ws.TargetMs)
		return
	}

	if ws.CurrentConfirmedMs >= ws.TargetMs || target-ws.CurrentConfirmedMs <= 1 {
		return
	}

	ws.start = time.Now()
	ws.Init = ws.CurrentConfirmedMs
	ws.PreviousCheckpoint = ws.CurrentConfirmedMs
	advancementRange := ws.advanceCheckpoint()
	ws.Events.Start.Trigger(ws.TargetMs, ws.CurrentCheckpoint, advancementRange)
}

// AddReferencedMessagesCount adds the amount of referenced messages to collect stats.
func (ws *WarpSync) AddReferencedMessagesCount(referencedMessagesCount int) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

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
	if ws.CurrentConfirmedMs+advRange >= ws.TargetMs || ws.CurrentCheckpoint+advRange >= ws.TargetMs {
		deltaRange := ws.TargetMs - ws.CurrentCheckpoint
		ws.CurrentCheckpoint = ws.TargetMs
		return int32(deltaRange)
	}

	// at start simply advance from the current confirmed
	if ws.CurrentCheckpoint == 0 {
		ws.CurrentCheckpoint = ws.CurrentConfirmedMs + advRange
		return int32(ws.AdvancementRange)
	}

	ws.CurrentCheckpoint = ws.CurrentCheckpoint + advRange
	return int32(advRange)
}

// resets the warp sync.
func (ws *WarpSync) reset() {
	ws.CurrentConfirmedMs = 0
	ws.CurrentCheckpoint = 0
	ws.TargetMs = 0
	ws.Init = 0
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

	cachedMs := w.storage.GetCachedMilestoneOrNil(msIndex) // milestone +1
	if cachedMs == nil {
		panic(fmt.Sprintf("milestone %d wasn't found", msIndex))
	}

	milestoneMessageID := cachedMs.GetMilestone().MessageID
	cachedMs.Release(true) // message -1

	_ = dag.TraverseParentsOfMessage(w.storage, milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			mapKey := cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()

			_, previouslyTraversed := w.traversed[mapKey]
			if !previouslyTraversed {
				w.traversed[mapKey] = struct{}{}
			}

			return !cachedMsgMeta.GetMetadata().IsSolid() && !previouslyTraversed, nil
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
