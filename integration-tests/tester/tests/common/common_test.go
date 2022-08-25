//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package common_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/integration-tests/tester/framework"
)

// TestCommon boots up a statically peered network and then checks that all
// nodes are sync, meaning that they actually received milestones.
func TestCommon(t *testing.T) {
	n, err := f.CreateStaticNetwork("test_common", nil, framework.DefaultStaticPeeringLayout())
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer syncCtxCancel()

	assert.NoError(t, n.AwaitAllSync(syncCtx))
}
