package autopeering

import (
	"context"
	"testing"
	"time"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAutopeering creates an autopeered network and then checks whether all nodes are synced.
// This test exists merely as a sanity check to verify that nodes still can connect to each other and
// are able to synchronize.
func TestAutopeering(t *testing.T) {
	n, err := f.CreateAutopeeredNetwork("test_autopeering", 4, 2)
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, n.AwaitAllSync(syncCtx))
}
