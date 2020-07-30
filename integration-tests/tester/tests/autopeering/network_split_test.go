package autopeering

import (
	"context"
	"testing"
	"time"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNetworkSplit boots up an autopeered network with two partitions, then verifies that all nodes
// indeed are only peered with other nodes from the same partition, then deletes the partitions and checks
// that nodes peer with the nodes from the other partition, finally, it verifies that all nodes are synced.
func TestNetworkSplit(t *testing.T) {
	n, err := f.CreateNetworkWithPartitions("autopnetworksplit", 6, 2, 2)
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	// test that nodes only have neighbors from same partition
	for _, partition := range n.Partitions() {
		for _, peer := range partition.Peers() {
			peers, err := peer.DebugWebAPI.Neighbors()
			require.NoError(t, err)
			require.Len(t, peers, 2, "should only be connected to %d neighbors", 2)

			// check that all neighbors are indeed in the same partition
			for _, p := range peers {
				assert.Contains(t, partition.PeersMap(), p.AutopeeringID)
			}
		}
	}

	require.NoError(t, n.DeletePartitions())

	// let them mingle and check that they all peer with each other
	require.NoError(t, n.AwaitPeering(3))

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, n.AwaitAllSync(syncCtx))
}
