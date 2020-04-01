package warpsync_test

import (
	"testing"

	"github.com/gohornet/hornet/pkg/protocol/warpsync"
	"github.com/stretchr/testify/assert"
)

func TestAdvanceAtEightyPercentReached(t *testing.T) {
	f := warpsync.AdvanceAtPercentageReached(0.8)
	assert.False(t, f(0, 0, 10))
	assert.False(t, f(5, 0, 10))
	assert.True(t, f(8, 0, 10))
}

func TestWarpSync_Update(t *testing.T) {
	ws := warpsync.New(50, warpsync.AdvanceAtPercentageReached(0.8))

	ws.UpdateCurrent(100)
	ws.UpdateTarget(1000)

	assert.EqualValues(t, ws.CurrentSolidMs, 100)
	assert.EqualValues(t, ws.CurrentCheckpoint, 150)

	// nothing should change besides current solid
	ws.UpdateCurrent(120)
	assert.EqualValues(t, ws.CurrentSolidMs, 120)
	assert.EqualValues(t, ws.CurrentCheckpoint, 150)

	// nothing should change besides current solid
	ws.UpdateCurrent(130)
	assert.EqualValues(t, ws.CurrentSolidMs, 130)
	assert.EqualValues(t, ws.CurrentCheckpoint, 150)

	// 80% reached
	ws.UpdateCurrent(140)
	assert.EqualValues(t, ws.CurrentSolidMs, 140)
	assert.EqualValues(t, ws.CurrentCheckpoint, 200)

	// shouldn't update anything - simulates non synced peer sending heartbeat
	ws.UpdateTarget(850)
	assert.EqualValues(t, ws.CurrentSolidMs, 140)
	assert.EqualValues(t, ws.CurrentCheckpoint, 200)
}
