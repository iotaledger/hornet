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
	n, err := f.CreateStaticNetwork("test_common", framework.DefaultStaticPeeringLayout)
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, n.AwaitAllSync(syncCtx))
}
