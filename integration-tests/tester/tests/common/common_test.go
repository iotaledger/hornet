package common

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommon boots up a statically peered network and then checks that all
// nodes are sync, meaning that they actually received milestones.
func TestCommon(t *testing.T) {
	n, err := f.CreateStaticNetwork("test_common", framework.DefaultStaticPeeringLayout)
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, n.AwaitAllSync(syncCtx))
	assert.NoError(t, n.TakeCPUProfiles(5))
	assert.NoError(t, n.TakeHeapSnapshots())

	duration := 30 * time.Second

	go func() {
		assert.NoError(t, n.SpamZeroVal(duration, runtime.NumCPU(), 50))
	}()

	assert.NoError(t, n.Coordinator().GraphMetrics(duration))
}
