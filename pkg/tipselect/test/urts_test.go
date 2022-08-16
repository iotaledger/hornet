//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/pkg/testsuite"
	"github.com/iotaledger/hornet/v2/pkg/tipselect"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	ProtocolVersion                         = 2
	MaxDeltaBlockYoungestConeRootIndexToCMI = 8
	MaxDeltaBlockOldestConeRootIndexToCMI   = 13
	BelowMaxDepth                           = 15
	RetentionRulesTipsLimitNonLazy          = 100
	MaxReferencedTipAgeNonLazy              = 3 * time.Second
	MaxChildrenNonLazy                      = 100
	RetentionRulesTipsLimitSemiLazy         = 20
	MaxReferencedTipAgeSemiLazy             = 3 * time.Second
	MaxChildrenSemiLazy                     = 100
	MinPoWScore                             = 1.0
)

func TestTipSelect(t *testing.T) {

	te := testsuite.SetupTestEnvironment(t, &iotago.Ed25519Address{}, 0, ProtocolVersion, BelowMaxDepth, MinPoWScore, false)
	defer te.CleanupTestEnvironment(true)

	serverMetrics := metrics.ServerMetrics{}

	calculator := tangle.NewTipScoreCalculator(te.Storage(), MaxDeltaBlockYoungestConeRootIndexToCMI, MaxDeltaBlockOldestConeRootIndexToCMI, BelowMaxDepth)

	ts := tipselect.New(
		context.Background(),
		calculator,
		te.SyncManager(),
		&serverMetrics,
		RetentionRulesTipsLimitNonLazy,
		MaxReferencedTipAgeNonLazy,
		uint32(MaxChildrenNonLazy),
		RetentionRulesTipsLimitSemiLazy,
		MaxReferencedTipAgeSemiLazy,
		uint32(MaxChildrenSemiLazy),
	)

	// fill the storage with some blocks to fill the tipselect pool
	blockCount := 0
	for i := 0; i < 100; i++ {
		blockMeta := te.NewTestBlock(blockCount, te.LastMilestoneParents())
		ts.AddTip(blockMeta)
		blockCount++
	}

	for i := 0; i < 1000; i++ {
		tips, err := ts.SelectNonLazyTips()
		require.NoError(te.TestInterface, err)
		require.NotNil(te.TestInterface, tips)

		require.GreaterOrEqual(te.TestInterface, len(tips), 1)
		require.LessOrEqual(te.TestInterface, len(tips), 8)

		cmi := te.SyncManager().ConfirmedMilestoneIndex()

		for _, tip := range tips {
			// we walk the cone of every tip to check the youngest and oldest milestone index it references
			var youngestConeRootIndex iotago.MilestoneIndex = 0
			var oldestConeRootIndex iotago.MilestoneIndex = math.MaxUint32

			updateIndexes := func(ycri iotago.MilestoneIndex, ocri iotago.MilestoneIndex) {
				if youngestConeRootIndex < ycri {
					youngestConeRootIndex = ycri
				}
				if oldestConeRootIndex > ocri {
					oldestConeRootIndex = ocri
				}
			}

			err := dag.TraverseParentsOfBlock(
				context.Background(),
				te.Storage(),
				tip,
				// traversal stops if no more blocks pass the given condition
				// Caution: condition func is not in DFS order
				func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
					defer cachedBlockMeta.Release(true) // meta -1

					// first check if the block was referenced => update ycri and ocri with the confirmation index
					if referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex(); referenced {
						updateIndexes(at, at)

						return false, nil
					}

					return true, nil
				},
				// consumer
				nil,
				// called on missing parents
				// return error on missing parents
				nil,
				// called on solid entry points
				func(blockID iotago.BlockID) error {
					// if the parent is a solid entry point, use the index of the solid entry point as ORTSI
					at, _, err := te.Storage().SolidEntryPointsIndex(blockID)
					if err != nil {
						return err
					}
					updateIndexes(at, at)

					return nil
				}, false)
			require.NoError(te.TestInterface, err)

			minOldestConeRootIndex := iotago.MilestoneIndex(0)
			if cmi > syncmanager.MilestoneIndexDelta(MaxDeltaBlockOldestConeRootIndexToCMI) {
				minOldestConeRootIndex = cmi - syncmanager.MilestoneIndexDelta(MaxDeltaBlockOldestConeRootIndexToCMI)
			}

			minYoungestConeRootIndex := iotago.MilestoneIndex(0)
			if cmi > syncmanager.MilestoneIndexDelta(MaxDeltaBlockYoungestConeRootIndexToCMI) {
				minYoungestConeRootIndex = cmi - syncmanager.MilestoneIndexDelta(MaxDeltaBlockYoungestConeRootIndexToCMI)
			}

			require.GreaterOrEqual(te.TestInterface, oldestConeRootIndex, minOldestConeRootIndex)
			require.LessOrEqual(te.TestInterface, oldestConeRootIndex, cmi)

			require.GreaterOrEqual(te.TestInterface, youngestConeRootIndex, minYoungestConeRootIndex)
			require.LessOrEqual(te.TestInterface, youngestConeRootIndex, cmi)
		}

		blockMeta := te.NewTestBlock(blockCount, tips)
		ts.AddTip(blockMeta)
		blockCount++

		if i%10 == 0 {
			// Issue a new milestone every 10 blocks
			conf, _ := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockMeta.BlockID()}, false)
			_ = dag.UpdateConeRootIndexes(context.Background(), te.Storage(), conf.Mutations.ReferencedBlocks.BlockIDs(), conf.MilestoneIndex)
			_, err := ts.UpdateScores()
			require.NoError(t, err)
		}
	}

	require.Equal(te.TestInterface, 1+100, len(te.Milestones)) // genesis + all created milestones
}
