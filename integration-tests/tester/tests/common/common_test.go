package common

import (
	"context"
	"testing"
	"time"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommon boots up a statically peered network and then checks that all
// nodes are sync, meaning that they actually received milestones.
func TestCommon(t *testing.T) {
	staticNet, err := f.CreateStaticNetwork("test_common", framework.DefaultStaticPeeringLayout)
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, staticNet)

	peeringCtx, peeringCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer peeringCtxCancel()
	assert.NoError(t, staticNet.AwaitPeering(peeringCtx))

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, staticNet.AwaitAllSync(syncCtx))
}
