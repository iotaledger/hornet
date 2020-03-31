package warpsync

import (
	"fmt"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/events"
)

// New creates a new WarpSync instance.
func New(rangePerCheckpoint int) *WarpSync {
	ws := &WarpSync{
		Events: Events{
			CheckpointUpdated: events.NewEvent(CheckpointCaller),
			Start:             events.NewEvent(SyncStartCaller),
			Done:              events.NewEvent(SyncDoneCaller),
		},
		current:         0,
		target:          0,
		checkpointRange: rangePerCheckpoint,
	}
	return ws
}

// WarpSync is metadata about doing a synchronization via STING messages.
type WarpSync struct {
	mu sync.Mutex
	Events
	start           time.Time
	init            milestone.Index
	current         milestone.Index
	checkpoint      milestone.Index
	target          milestone.Index
	checkpointRange int
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

// Update updates the WarpSync state.
func (ws *WarpSync) Update(current milestone.Index, target ...milestone.Index) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// prevent warp sync during normal operation when the node is already synced
	if ws.checkpoint == 0 && len(target) == 0 ||
		(len(target) > 0 && target[0]-current <= 1) {
		return
	}

	if len(target) != 0 && ws.target != 0 {
		if ws.target < target[0] {
			ws.target = target[0]
		}
		return
	}

	if current >= ws.current {
		ws.current = current
	}

	// we're over the checkpoint, trigger a milestone request trigger
	if ws.checkpoint != 0 {
		currentToCheckpointDelta := ws.checkpoint - ws.current

		// advance checkpoint when when're over/equal the checkpoint.
		// as an optimization we also update the checkpoint when we're at half the range
		// in order to have a continuous stream of solidifications
		if int(currentToCheckpointDelta) >= ws.checkpointRange/2 || ws.current >= ws.checkpoint {
			// advance checkpoint
			oldCheckpoint := ws.checkpoint
			if msRange := ws.advanceCheckpoint(); msRange != 0 {
				ws.Events.CheckpointUpdated.Trigger(ws.checkpoint, oldCheckpoint, msRange)
			}
		}
	}

	// done
	if ws.current >= ws.target && ws.target != 0 {
		ws.Events.Done.Trigger(int(ws.target-ws.init), time.Since(ws.start))
		ws.reset()
		return
	}

	if len(target) == 0 {
		return
	}

	// auto. set on first call
	if ws.target == 0 {
		ws.target = target[0]
	}

	// set checkpoint for the first time
	if ws.checkpoint == 0 && ws.target != 0 && ws.current < ws.target {
		ws.start = time.Now()
		ws.init = ws.current
		msRange := ws.advanceCheckpoint()
		ws.Events.Start.Trigger(ws.target, ws.checkpoint, msRange)
	}

	if target[0] <= ws.target {
		return
	}
	ws.target = target[0]
}

// advances the next checkpoint by either incrementing from the current
// via the checkpoint range or max to the target of the synchronization.
// returns the chosen range.
func (ws *WarpSync) advanceCheckpoint() int32 {
	fmt.Println("old", ws.checkpoint)
	msRange := milestone.Index(ws.checkpointRange)
	if ws.current+msRange > ws.target {
		ws.checkpoint = ws.target
		msRange = ws.target - ws.current
	} else {
		if ws.checkpoint == 0 {
			ws.checkpoint = ws.current + msRange
		} else {
			ws.checkpoint = ws.checkpoint + msRange
		}
	}
	fmt.Println("new", ws.checkpoint)
	return int32(msRange)
}

// resets the warp sync.
func (ws *WarpSync) reset() {
	ws.current = 0
	ws.checkpoint = 0
	ws.target = 0
	ws.init = 0
}
