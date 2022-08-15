//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package gossip_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
)

func TestAdvanceAtEightyPercentReached(t *testing.T) {
	f := gossip.AdvanceAtPercentageReached(0.8)
	assert.False(t, f(0, 0, 10))
	assert.False(t, f(5, 0, 10))
	assert.True(t, f(8, 0, 10))
}

func TestWarpSync_Update(t *testing.T) {
	ws := gossip.NewWarpSync(50, gossip.AdvanceAtPercentageReached(0.8))

	ws.UpdateCurrentConfirmedMilestone(100)
	ws.UpdateTargetMilestone(1000)

	assert.EqualValues(t, ws.CurrentConfirmedMilestone, 100)
	assert.EqualValues(t, ws.CurrentCheckpoint, 150)

	// nothing should change besides current confirmed
	ws.UpdateCurrentConfirmedMilestone(120)
	assert.EqualValues(t, ws.CurrentConfirmedMilestone, 120)
	assert.EqualValues(t, ws.CurrentCheckpoint, 150)

	// nothing should change besides current confirmed
	ws.UpdateCurrentConfirmedMilestone(130)
	assert.EqualValues(t, ws.CurrentConfirmedMilestone, 130)
	assert.EqualValues(t, ws.CurrentCheckpoint, 150)

	// 80% reached
	ws.UpdateCurrentConfirmedMilestone(140)
	assert.EqualValues(t, ws.CurrentConfirmedMilestone, 140)
	assert.EqualValues(t, ws.CurrentCheckpoint, 200)

	// shouldn't update anything - simulates non synced peer sending heartbeat
	ws.UpdateTargetMilestone(850)
	assert.EqualValues(t, ws.CurrentConfirmedMilestone, 140)
	assert.EqualValues(t, ws.CurrentCheckpoint, 200)
}
