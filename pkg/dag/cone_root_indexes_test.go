package dag_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	BelowMaxDepth = 5
	MinPoWScore   = 10.0
)

func TestConeRootIndexes(t *testing.T) {

	te := testsuite.SetupTestEnvironment(t, &iotago.Ed25519Address{}, 0, BelowMaxDepth, MinPoWScore, false)
	defer te.CleanupTestEnvironment(true)

	initBlocksCount := 10
	milestonesCount := 30
	minBlocksPerMilestone := 10
	maxBlocksPerMilestone := 100

	// build a tangle with 30 milestones and 10 - 100 blocks between the milestones
	_, _ = te.BuildTangle(initBlocksCount, BelowMaxDepth, milestonesCount, minBlocksPerMilestone, maxBlocksPerMilestone,
		nil,
		func(blockIDs hornet.BlockIDs, blockIDsPerMilestones []hornet.BlockIDs) hornet.BlockIDs {
			return hornet.BlockIDs{blockIDs[len(blockIDs)-1]}
		},
		func(msIndex milestone.Index, blockIDs hornet.BlockIDs, _ *whiteflag.Confirmation, _ *whiteflag.ConfirmedMilestoneStats) {
			latestMilestone := te.Milestones[len(te.Milestones)-1]
			cmi := latestMilestone.Milestone().Index()

			cachedBlockMeta := te.Storage().CachedBlockMetadataOrNil(blockIDs[len(blockIDs)-1])
			ycri, ocri, err := dag.ConeRootIndexes(context.Background(), te.Storage(), cachedBlockMeta, cmi)
			require.NoError(te.TestInterface, err)

			minOldestConeRootIndex := milestone.Index(1)
			if cmi > milestone.Index(BelowMaxDepth) {
				minOldestConeRootIndex = cmi - milestone.Index(BelowMaxDepth)
			}

			require.GreaterOrEqual(te.TestInterface, uint32(ocri), uint32(minOldestConeRootIndex))
			require.LessOrEqual(te.TestInterface, uint32(ocri), uint32(msIndex))

			require.GreaterOrEqual(te.TestInterface, uint32(ycri), uint32(minOldestConeRootIndex))
			require.LessOrEqual(te.TestInterface, uint32(ycri), uint32(msIndex))
		},
	)

	latestMilestone := te.Milestones[len(te.Milestones)-1]
	cmi := latestMilestone.Milestone().Index()

	// Use Null hash and last milestone hash as parents
	parents := append(latestMilestone.Milestone().Parents(), hornet.NullBlockID())
	msg := te.NewBlockBuilder("below max depth").Parents(parents.RemoveDupsAndSortByLexicalOrder()).BuildTaggedData().Store()

	cachedBlockMeta := te.Storage().CachedBlockMetadataOrNil(msg.StoredBlockID())
	ycri, ocri, err := dag.ConeRootIndexes(context.Background(), te.Storage(), cachedBlockMeta, cmi)
	require.NoError(te.TestInterface, err)

	// NullHash is SEP for index 0
	require.Equal(te.TestInterface, uint32(0), uint32(ocri))
	require.LessOrEqual(te.TestInterface, uint32(ycri), uint32(cmi))
}
