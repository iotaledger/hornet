package test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/tipselect"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	MaxDeltaMsgYoungestConeRootIndexToCMI = 8
	MaxDeltaMsgOldestConeRootIndexToCMI   = 13
	BelowMaxDepth                         = 15
	RetentionRulesTipsLimitNonLazy        = 100
	MaxReferencedTipAgeNonLazy            = 3 * time.Second
	MaxChildrenNonLazy                    = 100
	SpammerTipsThresholdNonLazy           = 0
	RetentionRulesTipsLimitSemiLazy       = 20
	MaxReferencedTipAgeSemiLazy           = 3 * time.Second
	MaxChildrenSemiLazy                   = 100
	SpammerTipsThresholdSemiLazy          = 30
	MinPowScore                           = 1.0
)

func TestTipSelect(t *testing.T) {

	te := testsuite.SetupTestEnvironment(t, &iotago.Ed25519Address{}, 0, BelowMaxDepth, MinPowScore, false)
	defer te.CleanupTestEnvironment(true)

	serverMetrics := metrics.ServerMetrics{}

	ts := tipselect.New(
		te.Storage(),
		&serverMetrics,
		MaxDeltaMsgYoungestConeRootIndexToCMI,
		MaxDeltaMsgOldestConeRootIndexToCMI,
		BelowMaxDepth,
		RetentionRulesTipsLimitNonLazy,
		MaxReferencedTipAgeNonLazy,
		uint32(MaxChildrenNonLazy),
		SpammerTipsThresholdNonLazy,
		RetentionRulesTipsLimitSemiLazy,
		MaxReferencedTipAgeSemiLazy,
		uint32(MaxChildrenSemiLazy),
		SpammerTipsThresholdSemiLazy,
	)

	// fill the storage with some messages to fill the tipselect pool
	msgCount := 0
	for i := 0; i < 100; i++ {
		msg := te.NewMessageBuilder(fmt.Sprintf("%d", msgCount)).Parents(hornet.MessageIDs{te.Milestones[0].GetMilestone().MessageID}).BuildIndexation().Store()
		cachedMsgMeta := te.Storage().GetCachedMessageMetadataOrNil(msg.StoredMessageID()) // metadata +1
		cachedMsgMeta.ConsumeMetadata(ts.AddTip)                                           // metadata -1
		msgCount++
	}

	for i := 0; i < 1000; i++ {
		tips, err := ts.SelectNonLazyTips()
		require.NoError(te.TestState, err)
		require.NotNil(te.TestState, tips)

		require.GreaterOrEqual(te.TestState, len(tips), 1)
		require.LessOrEqual(te.TestState, len(tips), 8)

		cmi := te.Storage().GetConfirmedMilestoneIndex()

		for _, tip := range tips {
			// we walk the cone of every tip to check the youngest and oldest milestone index it references
			var youngestConeRootIndex milestone.Index = 0
			var oldestConeRootIndex milestone.Index = 0

			youngestConeRootIndex = 0
			oldestConeRootIndex = math.MaxUint32

			updateIndexes := func(ycri milestone.Index, ocri milestone.Index) {
				if youngestConeRootIndex < ycri {
					youngestConeRootIndex = ycri
				}
				if oldestConeRootIndex > ocri {
					oldestConeRootIndex = ocri
				}
			}

			err := dag.TraverseParentsOfMessage(te.Storage(), tip,
				// traversal stops if no more messages pass the given condition
				// Caution: condition func is not in DFS order
				func(cachedMetadata *storage.CachedMetadata) (bool, error) { // meta +1
					defer cachedMetadata.Release(true) // meta -1

					// first check if the msg was referenced => update ycri and ocri with the confirmation index
					if referenced, at := cachedMetadata.GetMetadata().GetReferenced(); referenced {
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
				func(messageID hornet.MessageID) {
					// if the parent is a solid entry point, use the index of the solid entry point as ORTSI
					at, _ := te.Storage().SolidEntryPointsIndex(messageID)
					updateIndexes(at, at)
				}, false, nil)
			require.NoError(te.TestState, err)

			minOldestConeRootIndex := milestone.Index(1)
			if cmi > milestone.Index(MaxDeltaMsgOldestConeRootIndexToCMI) {
				minOldestConeRootIndex = cmi - milestone.Index(MaxDeltaMsgOldestConeRootIndexToCMI)
			}

			minYoungestConeRootIndex := milestone.Index(1)
			if cmi > milestone.Index(MaxDeltaMsgYoungestConeRootIndexToCMI) {
				minYoungestConeRootIndex = cmi - milestone.Index(MaxDeltaMsgYoungestConeRootIndexToCMI)
			}

			require.GreaterOrEqual(te.TestState, uint32(oldestConeRootIndex), uint32(minOldestConeRootIndex))
			require.LessOrEqual(te.TestState, uint32(oldestConeRootIndex), uint32(cmi))

			require.GreaterOrEqual(te.TestState, uint32(youngestConeRootIndex), uint32(minYoungestConeRootIndex))
			require.LessOrEqual(te.TestState, uint32(youngestConeRootIndex), uint32(cmi))
		}

		msg := te.NewMessageBuilder(fmt.Sprintf("%d", msgCount)).Parents(tips).BuildIndexation().Store()
		cachedMsgMeta := te.Storage().GetCachedMessageMetadataOrNil(msg.StoredMessageID()) // metadata +1
		cachedMsgMeta.ConsumeMetadata(ts.AddTip)                                           // metadata -1
		msgCount++

		if i%10 == 0 {
			// Issue a new milestone every 10 messages
			conf, _ := te.IssueAndConfirmMilestoneOnTip(msg.StoredMessageID(), false)
			dag.UpdateConeRootIndexes(te.Storage(), conf.Mutations.MessagesReferenced, conf.MilestoneIndex)
			ts.UpdateScores()
		}
	}

	require.Equal(te.TestState, 1+100, len(te.Milestones)) // genesis + all created milestones
}
