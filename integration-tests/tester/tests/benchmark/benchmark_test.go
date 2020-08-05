package common

import (
	"context"
	"testing"
	"time"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkBenchmark boots up a statically peered network and then graphs TPS, CPU and memory profiles
// while the network is sustaining a high inflow of transactions.
func TestNetworkBenchmark(t *testing.T) {
	n, err := f.CreateStaticNetwork("test_common", framework.DefaultStaticPeeringLayout)
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, n.AwaitAllSync(syncCtx))
}
