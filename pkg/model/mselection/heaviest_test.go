package mselection

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	iotago "github.com/iotaledger/iota.go/v2"
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
	lastMsgID := hornet.GetNullMessageID()
	for i := 1; i <= numTestMsgs; i++ {
		msg := te.NewTestMessage(i, hornet.MessageIDs{lastMsgID})
		hps.OnNewSolidMessage(msg)
		lastMsgID = msg.GetMessageID()
	}

	tips, err := hps.SelectTips(1)
	assert.NoError(t, err)
	assert.Len(t, tips, 1)

	// check if the tip on top was picked
	assert.ElementsMatch(t, lastMsgID, tips[0])

	// check if trackedMessages are resetted after tipselect
	assert.Len(t, hps.trackedMessages, 0)
}

func TestHeaviestSelector_SelectTipsChains(t *testing.T) {
	te, hps := initTest(t)
	defer te.CleanupTestEnvironment(true)

	numChains := 2
	lastMsgIDs := make(hornet.MessageIDs, 2)
	for i := 0; i < numChains; i++ {
		lastMsgIDs[i] = hornet.GetNullMessageID()
		for j := 1; j <= numTestMsgs; j++ {
			msgMeta := te.NewTestMessage(i*numTestMsgs+j, hornet.MessageIDs{lastMsgIDs[i]})
			hps.OnNewSolidMessage(msgMeta)
			lastMsgIDs[i] = msgMeta.GetMessageID()
		}
	}

	// check if all messages are tracked
	assert.Equal(t, numChains*numTestMsgs, hps.GetTrackedMessagesCount())

	tips, err := hps.SelectTips(2)
	assert.NoError(t, err)
	assert.Len(t, tips, 2)

	// check if the tips on top of both branches were picked
	assert.ElementsMatch(t, lastMsgIDs, tips)

	// check if trackedMessages are resetted after tipselect
	assert.Len(t, hps.trackedMessages, 0)
}

func BenchmarkHeaviestSelector_OnNewSolidMessage(b *testing.B) {
	te, hps := initTest(b)
	defer te.CleanupTestEnvironment(true)

	msgIDs := hornet.MessageIDs{hornet.GetNullMessageID()}
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
		msgIDs = append(msgIDs, msgs[i].GetMessageID())
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hps.OnNewSolidMessage(msgs[i%numBenchmarkMsgs])
	}
	hps.reset()
}
