package test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/tipselect"
)

const (
	MaxDeltaMsgYoungestConeRootIndexToLSMI = 8
	MaxDeltaMsgOldestConeRootIndexToLSMI   = 13
	BelowMaxDepth                          = 15
	RetentionRulesTipsLimitNonLazy         = 100
	MaxReferencedTipAgeSecondsNonLazy      = 3
	MaxChildrenNonLazy                     = 100
	SpammerTipsThresholdNonLazy            = 0
	RetentionRulesTipsLimitSemiLazy        = 20
	MaxReferencedTipAgeSecondsSemiLazy     = 3
	MaxChildrenSemiLazy                    = 100
	SpammerTipsThresholdSemiLazy           = 30
)

var (
	seed1, _ = hex.DecodeString("96d9ff7a79e4b0a5f3e5848ae7867064402da92a62eabb4ebbe463f12d1f3b1aace1775488f51cb1e3a80732a03ef60b111d6833ab605aa9f8faebeb33bbe3d9")
)

func TestTipSelect(t *testing.T) {

	genesisWallet := utils.NewHDWallet("Seed1", seed1, 0)
	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 0, false)
	defer te.CleanupTestEnvironment(true)

	serverMetrics := metrics.ServerMetrics{}

	ts := tipselect.New(
		te.Storage(),
		&serverMetrics,
		MaxDeltaMsgYoungestConeRootIndexToLSMI,
		MaxDeltaMsgOldestConeRootIndexToLSMI,
		BelowMaxDepth,
		RetentionRulesTipsLimitNonLazy,
		MaxReferencedTipAgeSecondsNonLazy,
		uint32(MaxChildrenNonLazy),
		SpammerTipsThresholdNonLazy,
		RetentionRulesTipsLimitSemiLazy,
		MaxReferencedTipAgeSecondsSemiLazy,
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

		lsmi := te.Storage().GetSolidMilestoneIndex()

		for _, tip := range tips {
			// we walk the cone of every tip to check the youngest and oldest milestone index it references
			var youngestConeRootIndex milestone.Index = 0
			var oldestConeRootIndex milestone.Index = 0

			updateIndexes := func(ycri milestone.Index, ocri milestone.Index) {
				if (youngestConeRootIndex == 0) || (youngestConeRootIndex < ycri) {
					youngestConeRootIndex = ycri
				}
				if (oldestConeRootIndex == 0) || (oldestConeRootIndex > ocri) {
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

			minOldestConeRootIndex := lsmi
			if minOldestConeRootIndex > milestone.Index(MaxDeltaMsgOldestConeRootIndexToLSMI) {
				minOldestConeRootIndex -= milestone.Index(MaxDeltaMsgOldestConeRootIndexToLSMI)
			} else {
				minOldestConeRootIndex = 1
			}

			minYoungestConeRootIndex := lsmi
			if minYoungestConeRootIndex > milestone.Index(MaxDeltaMsgYoungestConeRootIndexToLSMI) {
				minYoungestConeRootIndex -= milestone.Index(MaxDeltaMsgYoungestConeRootIndexToLSMI)
			} else {
				minYoungestConeRootIndex = 1
			}

			require.GreaterOrEqual(te.TestState, uint32(oldestConeRootIndex), uint32(minOldestConeRootIndex))
			require.LessOrEqual(te.TestState, uint32(oldestConeRootIndex), uint32(lsmi))

			require.GreaterOrEqual(te.TestState, uint32(youngestConeRootIndex), uint32(minYoungestConeRootIndex))
			require.LessOrEqual(te.TestState, uint32(youngestConeRootIndex), uint32(lsmi))
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
