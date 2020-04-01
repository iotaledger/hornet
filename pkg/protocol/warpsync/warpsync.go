package warpsync

import (
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/events"
)

// New creates a new WarpSync instance with the given advancement range and criteria func.
// If no advancement func is provided, the WarpSync uses AdvanceAtPercentageReached with DefaultAdvancementThreshold.
func New(advRange int, advanceCheckpointCriteriaFunc ...AdvanceCheckpointCriteria) *WarpSync {
	ws := &WarpSync{
		Events: Events{
			CheckpointUpdated: events.NewEvent(CheckpointCaller),
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
	handler.(func(delta int, dur time.Duration))(params[0].(int), params[1].(time.Duration))
}

func CheckpointCaller(handler interface{}, params ...interface{}) {
	handler.(func(newCheckpoint milestone.Index, oldCheckpoint milestone.Index, msRange int32))(params[0].(milestone.Index), params[1].(milestone.Index), params[2].(int32))
}

// Events holds WarpSync related events.
type Events struct {
	// Fired when a new set of milestones should be requested.
	CheckpointUpdated *events.Event
	// Fired when warp synchronization starts.
	Start *events.Event
	// Fired when the warp synchronization is done.
	Done *events.Event
}

// AdvanceCheckpointCriteria is a function which determines whether the checkpoint should be advanced.
type AdvanceCheckpointCriteria func(currentSolid, previousCheckpoint, currentCheckpoint milestone.Index) bool

// DefaultAdvancementThreshold is the default threshold at which a checkpoint advancement is done.
// Per default an advancement is always done as soon the solid milestone enters the range between
// the previous and current checkpoint.
const DefaultAdvancementThreshold = 0.0

// AdvanceAtPercentageReached is an AdvanceCheckpointCriteria which advances the checkpoint
// when the current one was reached by >=X% by the current solid milestone in relation to the previous checkpoint.
func AdvanceAtPercentageReached(threshold float64) AdvanceCheckpointCriteria {
	return func(currentSolid, previousCheckpoint, currentCheckpoint milestone.Index) bool {
		// the previous checkpoint can be over the current solid one, as advancements
		// move the checkpoint window above the solid milestone
		if currentSolid < previousCheckpoint {
			return false
		}
		checkpointDelta := currentCheckpoint - previousCheckpoint
		progress := currentSolid - previousCheckpoint
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
	// The current solid milestone of the node.
	CurrentSolidMs milestone.Index
	// The target milestone to which to synchronize to.
	TargetMs milestone.Index
	// The previous checkpoint of the synchronization.
	PreviousCheckpoint milestone.Index
	// The current checkpoint of the synchronization.
	CurrentCheckpoint milestone.Index
	// The used advancement range per checkpoint.
	AdvancementRange int
}

// UpdateCurrent updates the current solid milestone index state.
func (ws *WarpSync) UpdateCurrent(current milestone.Index) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if current <= ws.CurrentSolidMs {
		return
	}
	ws.CurrentSolidMs = current

	// synchronization not started
	if ws.CurrentCheckpoint == 0 {
		return
	}

	// finished
	if ws.TargetMs != 0 && ws.CurrentSolidMs >= ws.TargetMs {
		ws.Events.Done.Trigger(int(ws.TargetMs-ws.Init), time.Since(ws.start))
		ws.reset()
		return
	}

	// check whether advancement criteria is fulfilled
	if !ws.advCheckpointCriteria(ws.CurrentSolidMs, ws.PreviousCheckpoint, ws.CurrentCheckpoint) {
		return
	}

	oldCheckpoint := ws.CurrentCheckpoint
	if msRange := ws.advanceCheckpoint(); msRange != 0 {
		ws.Events.CheckpointUpdated.Trigger(ws.CurrentCheckpoint, oldCheckpoint, msRange)
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

	if ws.CurrentCheckpoint != 0 || ws.CurrentSolidMs >= ws.TargetMs || target-ws.CurrentSolidMs <= 1 {
		return
	}

	ws.start = time.Now()
	ws.Init = ws.CurrentSolidMs
	ws.PreviousCheckpoint = ws.CurrentSolidMs
	advancementRange := ws.advanceCheckpoint()
	ws.Events.Start.Trigger(ws.TargetMs, ws.CurrentCheckpoint, advancementRange)
}

// advances the next checkpoint by either incrementing from the current
// via the checkpoint range or max to the target of the synchronization.
// returns the chosen range.
func (ws *WarpSync) advanceCheckpoint() int32 {
	if ws.CurrentCheckpoint != 0 {
		ws.PreviousCheckpoint = ws.CurrentCheckpoint
	}

	advRange := milestone.Index(ws.AdvancementRange)

	// if we reach the target, just take the delta from target to current solid
	if ws.CurrentSolidMs+advRange >= ws.TargetMs {
		ws.CurrentCheckpoint = ws.TargetMs
		return int32(ws.TargetMs - ws.CurrentSolidMs)
	}

	// at start simply advance from the current solid
	if ws.CurrentCheckpoint == 0 {
		ws.CurrentCheckpoint = ws.CurrentSolidMs + advRange
		return int32(ws.AdvancementRange)
	}

	ws.CurrentCheckpoint = ws.CurrentCheckpoint + advRange
	return int32(advRange)
}

// resets the warp sync.
func (ws *WarpSync) reset() {
	ws.CurrentSolidMs = 0
	ws.CurrentCheckpoint = 0
	ws.TargetMs = 0
	ws.Init = 0
}
