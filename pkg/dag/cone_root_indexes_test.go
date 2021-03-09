package dag_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/testsuite"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	BelowMaxDepth = 5
	MinPowScore   = 10.0
)

func TestConeRootIndexes(t *testing.T) {

	te := testsuite.SetupTestEnvironment(t, &iotago.Ed25519Address{}, 0, BelowMaxDepth, MinPowScore, false)
	defer te.CleanupTestEnvironment(true)

	messages := hornet.MessageIDs{hornet.GetNullMessageID()}
	messagesPerMilestones := make([]hornet.MessageIDs, 0)

	const initMessagesCount = 10

	getParents := func() hornet.MessageIDs {

		if len(messages) < initMessagesCount {
			// reference the first milestone at the beginning
			return hornet.MessageIDs{te.Milestones[0].GetMilestone().MessageID, hornet.GetNullMessageID()}
		}

		parents := hornet.MessageIDs{}
		for j := 2; j <= 2+rand.Intn(7); j++ {
			msIndex := rand.Intn(BelowMaxDepth)
			if msIndex > len(messagesPerMilestones)-1 {
				msIndex = rand.Intn(len(messagesPerMilestones))
			}
			milestoneMessages := messagesPerMilestones[len(messagesPerMilestones)-1-msIndex]
			if len(milestoneMessages) == 0 {
				// use the milestone hash
				parents = append(parents, te.Milestones[len(te.Milestones)-1-msIndex].GetMilestone().MessageID)
				continue
			}
			parents = append(parents, milestoneMessages[rand.Intn(len(milestoneMessages))])
		}
		parents = parents.RemoveDupsAndSortByLexicalOrder()

		return parents
	}

	// build a tangle with 30 milestones and 10 - 100 messages between the milestones
	for msIndex := 2; msIndex < 30; msIndex++ {
		messagesPerMilestones = append(messagesPerMilestones, hornet.MessageIDs{})

		msgsCount := 10 + rand.Intn(90)
		for msgCount := 0; msgCount < msgsCount; msgCount++ {
			msg := te.NewMessageBuilder(fmt.Sprintf("%d_%d", msIndex, msgCount)).Parents(getParents()).BuildIndexation().Store()

			messages = append(messages, msg.StoredMessageID())
			messagesPerMilestones[len(messagesPerMilestones)-1] = append(messagesPerMilestones[len(messagesPerMilestones)-1], msg.StoredMessageID())
		}

		// confirm the new cone
		te.IssueAndConfirmMilestoneOnTip(messages[len(messages)-1], false)

		latestMilestone := te.Milestones[len(te.Milestones)-1]
		cmi := latestMilestone.GetMilestone().Index

		cachedMsgMeta := te.Storage().GetCachedMessageMetadataOrNil(messages[len(messages)-1])
		ycri, ocri := dag.GetConeRootIndexes(te.Storage(), cachedMsgMeta, cmi)

		minOldestConeRootIndex := milestone.Index(1)
		if cmi > milestone.Index(BelowMaxDepth) {
			minOldestConeRootIndex = cmi - milestone.Index(BelowMaxDepth)
		}

		require.GreaterOrEqual(te.TestState, uint32(ocri), uint32(minOldestConeRootIndex))
		require.LessOrEqual(te.TestState, uint32(ocri), uint32(msIndex))

		require.GreaterOrEqual(te.TestState, uint32(ycri), uint32(minOldestConeRootIndex))
		require.LessOrEqual(te.TestState, uint32(ycri), uint32(msIndex))
	}

	latestMilestone := te.Milestones[len(te.Milestones)-1]
	cmi := latestMilestone.GetMilestone().Index

	// Use Null hash and last milestone hash as parents
	parents := hornet.MessageIDs{latestMilestone.GetMilestone().MessageID, hornet.GetNullMessageID()}
	msg := te.NewMessageBuilder("below max depth").Parents(parents.RemoveDupsAndSortByLexicalOrder()).BuildIndexation().Store()

	cachedMsgMeta := te.Storage().GetCachedMessageMetadataOrNil(msg.StoredMessageID())
	ycri, ocri := dag.GetConeRootIndexes(te.Storage(), cachedMsgMeta, cmi)

	// NullHash is SEP for index 0
	require.Equal(te.TestState, uint32(0), uint32(ocri))
	require.LessOrEqual(te.TestState, uint32(ycri), uint32(cmi))
}
