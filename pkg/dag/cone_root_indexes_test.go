//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package dag_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/testsuite"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	ProtocolVersion = 2
	BelowMaxDepth   = 5
	MinPoWScore     = 10
)

func TestConeRootIndexes(t *testing.T) {

	te := testsuite.SetupTestEnvironment(t, &iotago.Ed25519Address{}, 0, ProtocolVersion, BelowMaxDepth, MinPoWScore, false)
	defer te.CleanupTestEnvironment(true)

	initBlocksCount := 10
	milestonesCount := 30
	minBlocksPerMilestone := 10
	maxBlocksPerMilestone := 100

	// build a tangle with 30 milestones and 10 - 100 blocks between the milestones
	_, _ = te.BuildTangle(initBlocksCount, BelowMaxDepth, milestonesCount, minBlocksPerMilestone, maxBlocksPerMilestone,
		nil,
		func(blockIDs iotago.BlockIDs, blockIDsPerMilestones []iotago.BlockIDs) iotago.BlockIDs {
			return iotago.BlockIDs{blockIDs[len(blockIDs)-1]}
		},
		func(msIndex iotago.MilestoneIndex, blockIDs iotago.BlockIDs, _ *whiteflag.Confirmation, _ *whiteflag.ConfirmedMilestoneStats) {
			latestMilestone := te.Milestones[len(te.Milestones)-1]
			cmi := latestMilestone.Milestone().Index()

			cachedBlockMeta := te.Storage().CachedBlockMetadataOrNil(blockIDs[len(blockIDs)-1])
			ycri, ocri, err := dag.ConeRootIndexes(context.Background(), te.Storage(), cachedBlockMeta, cmi)
			require.NoError(te.TestInterface, err)

			minOldestConeRootIndex := iotago.MilestoneIndex(1)
			if cmi > syncmanager.MilestoneIndexDelta(BelowMaxDepth) {
				minOldestConeRootIndex = cmi - syncmanager.MilestoneIndexDelta(BelowMaxDepth)
			}

			require.GreaterOrEqual(te.TestInterface, ocri, minOldestConeRootIndex)
			require.LessOrEqual(te.TestInterface, ocri, msIndex)

			require.GreaterOrEqual(te.TestInterface, ycri, minOldestConeRootIndex)
			require.LessOrEqual(te.TestInterface, ycri, msIndex)
		},
	)

	latestMilestone := te.Milestones[len(te.Milestones)-1]
	cmi := latestMilestone.Milestone().Index()

	// Use Null hash and last milestone ID as parents
	parents := append(latestMilestone.Milestone().Parents(), iotago.EmptyBlockID())
	block := te.NewBlockBuilder("below max depth").Parents(parents.RemoveDupsAndSort()).BuildTaggedData().Store()

	cachedBlockMeta := te.Storage().CachedBlockMetadataOrNil(block.StoredBlockID())
	ycri, ocri, err := dag.ConeRootIndexes(context.Background(), te.Storage(), cachedBlockMeta, cmi)
	require.NoError(te.TestInterface, err)

	// NullHash is SEP for index 0
	require.Equal(te.TestInterface, iotago.MilestoneIndex(0), ocri)
	require.LessOrEqual(te.TestInterface, ycri, cmi)
}
