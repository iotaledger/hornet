package mselection

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold = 20
	CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint             = 10
	CfgCoordinatorTipselectRandomTipsPerCheckpoint                        = 3
	CfgCoordinatorTipselectHeaviestBranchSelectionTimeoutMilliseconds     = 100

	numTestMsgs      = 32 * 100
	numBenchmarkMsgs = 5000
	BelowMaxDepth    = 15
	MinPowScore      = 1.0
)

func init() {
	rand.Seed(0)
}

func initTest(testInterface testing.TB) (*testsuite.TestEnvironment, *HeaviestSelector) {

	te := testsuite.SetupTestEnvironment(testInterface, &iotago.Ed25519Address{}, 0, BelowMaxDepth, MinPowScore, false)

	hps := New(
		CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold,
		CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint,
		CfgCoordinatorTipselectRandomTipsPerCheckpoint,
		CfgCoordinatorTipselectHeaviestBranchSelectionTimeoutMilliseconds,
	)

	return te, hps
}

func TestHeaviestSelector_SelectTipsChain(t *testing.T) {
	te, hps := initTest(t)
	defer te.CleanupTestEnvironment(true)

	// create a chain
	lastMsgID := hornet.NullMessageID()
	for i := 1; i <= numTestMsgs; i++ {
		msg := te.NewTestMessage(i, hornet.MessageIDs{lastMsgID})
		hps.OnNewSolidMessage(msg)
		lastMsgID = msg.MessageID()
	}

	tips, err := hps.SelectTips(1)
	assert.NoError(t, err)
	assert.Len(t, tips, 1)

	// check if the tip on top was picked
	assert.ElementsMatch(t, lastMsgID, tips[0])

	// check if trackedMessages are resetted after tipselect
	assert.Len(t, hps.trackedMessages, 0)
}

func TestHeaviestSelector_CheckTipsRemoved(t *testing.T) {
	te, hps := initTest(t)
	defer te.CleanupTestEnvironment(true)

	count := 8

	messages := make(hornet.MessageIDs, count)
	for i := 0; i < count; i++ {
		msg := te.NewTestMessage(i, hornet.MessageIDs{hornet.NullMessageID()})
		hps.OnNewSolidMessage(msg)
		messages[i] = msg.MessageID()
	}

	// check if trackedMessages match the current count
	assert.Len(t, hps.trackedMessages, count)

	// check if the current tips match the current count
	list := hps.tipsToList()
	assert.Len(t, list.msgs, count)

	// issue a new message that references the old ones
	msg := te.NewTestMessage(count, messages)
	hps.OnNewSolidMessage(msg)

	// old tracked messages should remain, plus the new one
	assert.Len(t, hps.trackedMessages, count+1)

	// all old tips should be removed, except the new one
	list = hps.tipsToList()
	assert.Len(t, list.msgs, 1)

	// select a tip
	tips, err := hps.SelectTips(1)
	assert.NoError(t, err)
	assert.Len(t, tips, 1)

	// check if the tip on top was picked
	assert.ElementsMatch(t, msg.MessageID(), tips[0])

	// check if trackedMessages are resetted after tipselect
	assert.Len(t, hps.trackedMessages, 0)

	list = hps.tipsToList()
	assert.Len(t, list.msgs, 0)
}

func TestHeaviestSelector_SelectTipsChains(t *testing.T) {
	te, hps := initTest(t)
	defer te.CleanupTestEnvironment(true)

	numChains := 2
	lastMsgIDs := make(hornet.MessageIDs, 2)
	for i := 0; i < numChains; i++ {
		lastMsgIDs[i] = hornet.NullMessageID()
		for j := 1; j <= numTestMsgs; j++ {
			msgMeta := te.NewTestMessage(i*numTestMsgs+j, hornet.MessageIDs{lastMsgIDs[i]})
			hps.OnNewSolidMessage(msgMeta)
			lastMsgIDs[i] = msgMeta.MessageID()
		}
	}

	// check if all messages are tracked
	assert.Equal(t, numChains*numTestMsgs, hps.TrackedMessagesCount())

	tips, err := hps.SelectTips(2)
	assert.NoError(t, err)
	assert.Len(t, tips, 2)

	// check if the tips on top of both branches were picked
	assert.ElementsMatch(t, lastMsgIDs, tips)

	// check if trackedMessages are resetted after tipselect
	assert.Len(t, hps.trackedMessages, 0)
}

func TestHeaviestSelector_SelectTipsCheckThresholds(t *testing.T) {
	te, hps := initTest(t)
	defer te.CleanupTestEnvironment(true)

	initMessagesCount := 10
	milestonesCount := 30
	minMessagesPerMilestone := 10
	maxMessagesPerMilestone := 100

	getConeRootIndexes := func(tip hornet.MessageID) (milestone.Index, milestone.Index) {
		// we walk the cone of every tip to check the youngest and oldest milestone index it references
		var youngestConeRootIndex milestone.Index = 0
		var oldestConeRootIndex milestone.Index = math.MaxUint32

		updateIndexes := func(ycri milestone.Index, ocri milestone.Index) {
			if youngestConeRootIndex < ycri {
				youngestConeRootIndex = ycri
			}
			if oldestConeRootIndex > ocri {
				oldestConeRootIndex = ocri
			}
		}

		err := dag.TraverseParentsOfMessage(
			context.Background(),
			te.Storage(),
			tip,
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMetadata *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMetadata.Release(true) // meta -1

				// first check if the msg was referenced => update ycri and ocri with the confirmation index
				if referenced, at := cachedMetadata.Metadata().ReferencedWithIndex(); referenced {
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
			func(messageID hornet.MessageID) error {
				// if the parent is a solid entry point, use the index of the solid entry point as ORTSI
				at, _, err := te.Storage().SolidEntryPointsIndex(messageID)
				if err != nil {
					return err
				}
				updateIndexes(at, at)
				return nil
			}, false)
		require.NoError(te.TestInterface, err)

		return youngestConeRootIndex, oldestConeRootIndex
	}

	// isBelowMaxDepth checks the below max depth criteria for the given message.
	isBelowMaxDepth := func(msgMeta *storage.MessageMetadata) bool {

		cmi := te.SyncManager().ConfirmedMilestoneIndex()

		_, ocri := getConeRootIndexes(msgMeta.MessageID())

		// if the OCRI to CMI delta is over belowMaxDepth, then the tip is invalid.
		return (cmi - ocri) > milestone.Index(BelowMaxDepth)
	}

	checkTips := func(tips hornet.MessageIDs) {

		cmi := te.SyncManager().ConfirmedMilestoneIndex()

		for _, tip := range tips {
			_, ocri := getConeRootIndexes(tip)
			minOldestConeRootIndex := milestone.Index(0)
			if cmi > milestone.Index(BelowMaxDepth) {
				minOldestConeRootIndex = cmi - milestone.Index(BelowMaxDepth)
			}

			require.GreaterOrEqual(te.TestInterface, uint32(ocri), uint32(minOldestConeRootIndex))
			require.LessOrEqual(te.TestInterface, uint32(ocri), uint32(cmi))
		}
	}

	// build a tangle with 30 milestones and 10 - 100 messages between the milestones
	_, _ = te.BuildTangle(initMessagesCount, BelowMaxDepth-1, milestonesCount, minMessagesPerMilestone, maxMessagesPerMilestone,
		func(_ milestone.Index, msgMeta *storage.MessageMetadata) {

			if isBelowMaxDepth(msgMeta) {
				// ignore tips that are below max depth
				return
			}

			hps.OnNewSolidMessage(msgMeta)
		},
		func(_ hornet.MessageIDs, _ []hornet.MessageIDs) hornet.MessageIDs {
			tips, err := hps.SelectTips(1)
			if err == ErrNoTipsAvailable {
				err = nil
				latestMilestoneMessageID := te.Milestones[len(te.Milestones)-1].Milestone().MessageID
				tips = hornet.MessageIDs{latestMilestoneMessageID}
			}

			// check if trackedMessages are resetted after tipselect
			require.Len(t, hps.trackedMessages, 0)

			require.NoError(te.TestInterface, err)
			require.NotNil(te.TestInterface, tips)

			require.GreaterOrEqual(te.TestInterface, len(tips), 1)
			require.LessOrEqual(te.TestInterface, len(tips), CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint+CfgCoordinatorTipselectRandomTipsPerCheckpoint)

			checkTips(tips)

			return tips
		},
		func(_ milestone.Index, _ hornet.MessageIDs, conf *whiteflag.Confirmation, _ *whiteflag.ConfirmedMilestoneStats) {
			_ = dag.UpdateConeRootIndexes(context.Background(), te.Storage(), conf.Mutations.MessagesReferenced, conf.MilestoneIndex)
		},
	)

	// check if trackedMessages are resetted after tipselect
	require.Len(t, hps.trackedMessages, 0)
}

func BenchmarkHeaviestSelector_OnNewSolidMessage(b *testing.B) {
	te, hps := initTest(b)
	defer te.CleanupTestEnvironment(true)

	msgIDs := hornet.MessageIDs{hornet.NullMessageID()}
	msgs := make([]*storage.MessageMetadata, numBenchmarkMsgs)
	for i := 0; i < numBenchmarkMsgs; i++ {
		tipCount := 1 + rand.Intn(7)
		if tipCount > len(msgIDs) {
			tipCount = len(msgIDs)
		}
		tips := make(hornet.MessageIDs, tipCount)
		for j := 0; j < tipCount; j++ {
			tips[j] = msgIDs[rand.Intn(len(msgIDs))]
		}
		tips = tips.RemoveDupsAndSortByLexicalOrder()

		msgs[i] = te.NewTestMessage(i, tips)
		msgIDs = append(msgIDs, msgs[i].MessageID())
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hps.OnNewSolidMessage(msgs[i%numBenchmarkMsgs])
	}
	hps.Reset()
}
