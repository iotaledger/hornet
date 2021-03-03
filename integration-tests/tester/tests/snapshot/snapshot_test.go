package snapshot

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	"github.com/iotaledger/iota.go/consts"
	iotago "github.com/iotaledger/iota.go/v2"
)

// TestSnapshot boots up a statically peered network where the nodes consume
// a full snapshot and then a delta snapshot.
// The full snapshot retracts the state from the ledger index 3, back to its snapshot index 1,
// where as the delta snapshot builds up the state to its snapshot index 5.
// The delta snapshot therefore contains the milestone diffs 2-5 and the final ms diff
// outputs the 40'000'000 tokens to an output with all 9s as its ID (deducting 10'0000 from the treasury).
func TestSnapshot(t *testing.T) {
	n, err := f.CreateStaticNetwork("test_snapshot", nil, framework.DefaultStaticPeeringLayout(), func(index int, cfg *framework.NodeConfig) {
		// run network without a coordinator
		if index == 0 {
			cfg.Coordinator.Bootstrap = false
			cfg.Coordinator.RunAsCoo = false
			cfg.Plugins.Enabled = []string{}
		}
		// modify to use different snapshot files
		cfg.Snapshot.FullSnapshotFilePath = "/assets/test_full_snapshot.bin"
		cfg.Snapshot.DeltaSnapshotFilePath = "/assets/test_delta_snapshot.bin"
	})
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	var targetOutputID iotago.UTXOInputID
	for i := 0; i < len(targetOutputID); i++ {
		targetOutputID[i] = 9
	}

	const finalOutputAmount = 40_000_000

	// check that on each node, the total supply is on an output with ID 999..
	for _, node := range n.Nodes {
		require.Eventually(t, func() bool {
			res, err := node.DebugNodeAPIClient.OutputByID(targetOutputID)
			if err != nil {
				return false
			}
			output, err := res.Output()
			if err != nil {
				return false
			}
			return output.(*iotago.SigLockedSingleOutput).Amount == finalOutputAmount
		}, 30*time.Second, 100*time.Millisecond)
	}

	// check that the treasury output contains total supply - 40'000'000
	for _, node := range n.Nodes {
		require.Eventually(t, func() bool {
			res, err := node.DebugNodeAPIClient.Treasury()
			if err != nil {
				log.Println(err)
				return false
			}
			return res.Amount == consts.TotalSupply-40_000_000
		}, 30*time.Second, 100*time.Millisecond)
	}
}
