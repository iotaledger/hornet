package network

import (
	"testing"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkSplit(t *testing.T) {
	n, err := f.CreateNetworkWithPartitions("autopeering_TestNetworkSplit", 6, 2, 2)
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

	err = n.DeletePartitions()
	require.NoError(t, err)

	// let them mingle and check that they all peer with each other
	err = n.AwaitPeering(4)
	require.NoError(t, err)
}
