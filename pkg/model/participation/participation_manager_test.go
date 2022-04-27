package participation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/gohornet/hornet/pkg/model/participation/test"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
)

func TestEventStateHelpers(t *testing.T) {

	eventCommenceIndex := milestone.Index(90)
	eventStartIndex := milestone.Index(100)
	eventEndIndex := milestone.Index(200)

	eventBuilder := participation.NewEventBuilder("Test", eventCommenceIndex, eventStartIndex, eventEndIndex, "Sample")

	questionBuilder := participation.NewQuestionBuilder("Q1", "-")
	questionBuilder.AddAnswer(&participation.Answer{
		Value:          1,
		Text:           "A1",
		AdditionalInfo: "-",
	})
	questionBuilder.AddAnswer(&participation.Answer{
		Value:          2,
		Text:           "A2",
		AdditionalInfo: "-",
	})

	question, err := questionBuilder.Build()
	require.NoError(t, err)

	ballotBuilder := participation.NewBallotBuilder()
	ballotBuilder.AddQuestion(question)
	payload, err := ballotBuilder.Build()
	require.NoError(t, err)

	eventBuilder.Payload(payload)

	event, err := eventBuilder.Build()
	require.NoError(t, err)

	// Verify status
	require.Equal(t, "upcoming", event.Status(89))
	require.Equal(t, "commencing", event.Status(90))
	require.Equal(t, "commencing", event.Status(91))
	require.Equal(t, "holding", event.Status(100))
	require.Equal(t, "holding", event.Status(101))
	require.Equal(t, "holding", event.Status(199))
	require.Equal(t, "ended", event.Status(200))
	require.Equal(t, "ended", event.Status(201))

	// Verify IsAcceptingParticipation
	require.False(t, event.IsAcceptingParticipation(89))
	require.True(t, event.IsAcceptingParticipation(90))
	require.True(t, event.IsAcceptingParticipation(91))
	require.True(t, event.IsAcceptingParticipation(99))
	require.True(t, event.IsAcceptingParticipation(100))
	require.True(t, event.IsAcceptingParticipation(101))
	require.True(t, event.IsAcceptingParticipation(199))
	require.False(t, event.IsAcceptingParticipation(200))
	require.False(t, event.IsAcceptingParticipation(201))

	// Verify IsCountingParticipation
	require.False(t, event.IsCountingParticipation(89))
	require.False(t, event.IsCountingParticipation(90))
	require.False(t, event.IsCountingParticipation(91))
	require.False(t, event.IsCountingParticipation(99))
	require.True(t, event.IsCountingParticipation(100))
	require.True(t, event.IsCountingParticipation(101))
	require.True(t, event.IsCountingParticipation(199))
	require.False(t, event.IsCountingParticipation(200))
	require.False(t, event.IsCountingParticipation(201))

	// Verify ShouldAcceptParticipation
	require.False(t, event.ShouldAcceptParticipation(89))
	require.False(t, event.ShouldAcceptParticipation(90))
	require.True(t, event.ShouldAcceptParticipation(91))
	require.True(t, event.ShouldAcceptParticipation(99))
	require.True(t, event.ShouldAcceptParticipation(100))
	require.True(t, event.ShouldAcceptParticipation(101))
	require.True(t, event.ShouldAcceptParticipation(199))
	require.True(t, event.ShouldAcceptParticipation(200))
	require.False(t, event.ShouldAcceptParticipation(201))

	// Verify ShouldCountParticipation
	require.False(t, event.ShouldCountParticipation(89))
	require.False(t, event.ShouldCountParticipation(90))
	require.False(t, event.ShouldCountParticipation(91))
	require.False(t, event.ShouldCountParticipation(99))
	require.False(t, event.ShouldCountParticipation(100))
	require.True(t, event.ShouldCountParticipation(101))
	require.True(t, event.ShouldCountParticipation(199))
	require.True(t, event.ShouldCountParticipation(200))
	require.False(t, event.ShouldCountParticipation(201))
}

func TestEventStates(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	require.Empty(t, env.ParticipationManager().Events())
	eventID := env.StoreDefaultEvent(5, 1, 2)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(6), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(8), event.EndMilestoneIndex())

	// No participation should be running right now
	require.Equal(t, 1, len(env.ParticipationManager().Events()))
	env.AssertEventsCount(0, 0)

	env.IssueMilestone() // 5

	// Event should be accepting votes, but not counting
	require.Equal(t, 1, len(env.ParticipationManager().Events()))
	env.AssertEventsCount(1, 0)

	env.IssueMilestone() // 6

	// Event should be accepting and counting votes
	require.Equal(t, 1, len(env.ParticipationManager().Events()))
	env.AssertEventsCount(1, 1)

	env.IssueMilestone() // 7

	// Event should be accepting and counting votes
	require.Equal(t, 1, len(env.ParticipationManager().Events()))
	env.AssertEventsCount(1, 1)

	env.IssueMilestone() // 8

	// Event should be ended
	require.Equal(t, 1, len(env.ParticipationManager().Events()))
	env.AssertEventsCount(0, 0)
}

func TestTaggedDataPayloads(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	eventID := RandEventID()

	okMessage := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{10, 20, 30, 40},
		}).
		Build()

	noTaggedDataMessage := env.NewMessageBuilder().
		LatestMilestoneAsParents().
		FromWallet(env.Wallet2).
		ToWallet(env.Wallet2).
		Amount(env.Wallet2.Balance()).
		Build()

	invalidPayloadMessage := env.NewMessageBuilder(test.ParticipationTag).
		LatestMilestoneAsParents().
		FromWallet(env.Wallet2).
		ToWallet(env.Wallet2).
		Amount(env.Wallet2.Balance()).
		TagData([]byte{0}).
		Build()

	emptyTaggedDataMessage := env.NewMessageBuilder(test.ParticipationTag).
		LatestMilestoneAsParents().
		FromWallet(env.Wallet2).
		ToWallet(env.Wallet2).
		Amount(env.Wallet2.Balance()).
		Build()

	b := participation.NewParticipationsBuilder()
	b.AddParticipation(&participation.Participation{
		EventID: eventID,
		Answers: []byte{10, 20, 30, 40},
	})
	participations, err := b.Build()
	require.NoError(t, err)
	participationsData, err := participations.Serialize(serializer.DeSeriModePerformValidation, nil)
	require.NoError(t, err)

	wrongAddressMessage := env.NewMessageBuilder(test.ParticipationTag).
		LatestMilestoneAsParents().
		FromWallet(env.Wallet2).
		ToWallet(env.Wallet3).
		Amount(env.Wallet2.Balance()).
		TagData(participationsData).
		Build()

	multipleOutputsMessage := env.NewMessageBuilder(test.ParticipationTag).
		LatestMilestoneAsParents().
		FromWallet(env.Wallet2).
		ToWallet(env.Wallet3).
		Amount(10_000_000).
		TagData(participationsData).
		Build()

	txBuilder := builder.NewTransactionBuilder(env.ProtocolParameters().NetworkID())
	txBuilder.AddTaggedDataPayload(&iotago.TaggedData{
		Tag:  []byte(test.ParticipationTag),
		Data: participationsData,
	})
	txBuilder.AddInput(&builder.ToBeSignedUTXOInput{Address: env.Wallet3.Address(), OutputID: *env.Wallet3.Outputs()[0].OutputID(), Output: env.Wallet3.Outputs()[0].Output()})
	txBuilder.AddInput(&builder.ToBeSignedUTXOInput{Address: env.Wallet4.Address(), OutputID: *env.Wallet4.Outputs()[0].OutputID(), Output: env.Wallet4.Outputs()[0].Output()})
	txBuilder.AddOutput(&iotago.BasicOutput{Conditions: iotago.UnlockConditions{&iotago.AddressUnlockCondition{Address: env.Wallet4.Address()}}, Amount: env.Wallet3.Balance() + env.Wallet4.Balance()})
	wallet3PrivKey, _ := env.Wallet3.KeyPair()
	wallet4PrivKey, _ := env.Wallet4.KeyPair()
	inputAddrSigner := iotago.NewInMemoryAddressSigner(iotago.AddressKeys{Address: env.Wallet3.Address(), Keys: wallet3PrivKey}, iotago.AddressKeys{Address: env.Wallet4.Address(), Keys: wallet4PrivKey})
	msgBuilder := txBuilder.BuildAndSwapToMessageBuilder(env.ProtocolParameters(), inputAddrSigner, nil)
	msgBuilder.Parents(env.LastMilestoneParents().ToSliceOfSlices())

	msg, err := msgBuilder.Build()
	require.NoError(t, err)
	// Skipped PoW since we are not validating it anyway
	sweepAndParticipateMessage, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, env.ProtocolParameters())
	require.NoError(t, err)

	tests := []struct {
		name                string
		message             *storage.Message
		outputExists        bool
		participationsCount int
	}{
		{"ok", okMessage.StoredMessage(), true, 1},
		{"sweep and participate", sweepAndParticipateMessage, true, 1},
		{"no tag", noTaggedDataMessage.StoredMessage(), false, 0},
		{"invalid payload", invalidPayloadMessage.StoredMessage(), false, 0},
		{"empty tag", emptyTaggedDataMessage.StoredMessage(), false, 0},
		{"wrong address", wrongAddressMessage.StoredMessage(), false, 0},
		{"multiple outputs", multipleOutputsMessage.StoredMessage(), false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			output, participations, err := env.ParticipationManager().ParticipationsFromMessage(tt.message, 0)
			require.NoError(t, err)

			if tt.outputExists {
				require.NotNil(t, output)
				require.Equal(t, tt.participationsCount, len(participations))
			} else {
				require.Nil(t, output)
			}
		})
	}

}

func TestSingleBallotVote(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 3)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(10), event.EndMilestoneIndex())

	// Event should not be accepting votes yet
	require.Equal(t, 0, len(env.ParticipationManager().EventsAcceptingParticipation()))

	// Issue a vote and milestone
	env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 5

	// ParticipationPayload should not have been counted so far because it was not accepting votes yet
	status, err := env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 0, 0)

	// Event should be accepting votes now
	require.Equal(t, 1, len(env.ParticipationManager().EventsAcceptingParticipation()))

	// Participation again
	castVote := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 6

	// Event should be accepting votes, but the vote should not be weighted yet, just added to the current status
	env.AssertEventsCount(1, 0)

	status, err = env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 0)

	env.IssueMilestone() // 7

	// Event should be accepting and counting votes, but the vote we did before should not be weighted yet
	env.AssertEventsCount(1, 1)

	status, err = env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 0)

	env.IssueMilestone() // 8

	// Event should be accepting and counting votes, the vote should now be weighted
	env.AssertEventsCount(1, 1)

	status, err = env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 1_000)

	env.IssueMilestone() // 9

	// Event should be accepting and counting votes, the vote should now be weighted double
	env.AssertEventsCount(1, 1)

	status, err = env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 2_000)

	env.IssueMilestone() // 10

	// Event should be ended
	env.AssertEventsCount(0, 0)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 3_000)

	env.IssueMilestone() // 11

	var trackedVote *participation.TrackedParticipation
	trackedVote, err = env.ParticipationManager().ParticipationForOutputID(eventID, castVote.Message().GeneratedUTXO().OutputID())
	require.NoError(t, err)
	require.Equal(t, castVote.Message().StoredMessageID(), trackedVote.MessageID)
	require.Equal(t, milestone.Index(6), trackedVote.StartIndex)
	require.Equal(t, milestone.Index(11), trackedVote.EndIndex)

	var messageFromParticipationStore *storage.Message
	messageFromParticipationStore, err = env.ParticipationManager().MessageForEventAndMessageID(eventID, trackedVote.MessageID)
	require.NoError(t, err)
	require.NotNil(t, messageFromParticipationStore)
	require.Equal(t, messageFromParticipationStore.Message(), castVote.Message().IotaMessage())
}

func TestInvalidVoteHandling(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 3)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(10), event.EndMilestoneIndex())

	// Event should not be accepting votes yet
	require.Equal(t, 0, len(env.ParticipationManager().EventsAcceptingParticipation()))

	// Issue a vote and milestone
	env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 5

	// ParticipationPayload should not have been counted so far because it was not accepting votes yet
	status, err := env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 0, 0)

	// Event should be accepting votes now
	require.Equal(t, 1, len(env.ParticipationManager().EventsAcceptingParticipation()))

	// Participation again
	castVote := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 6

	// Event should be accepting votes, but the vote should not be weighted yet, just added to the current status
	env.AssertEventsCount(1, 0)

	status, err = env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 0)

	env.IssueMilestone() // 7

	// Event should be accepting and counting votes, but the vote we did before should not be weighted yet
	env.AssertEventsCount(1, 1)

	status, err = env.ParticipationManager().EventStatus(eventID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 0)

	env.IssueMilestone() // 8

	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 1_000)

	// Send an invalid participation
	invalidParticipation := env.NewMessageBuilder(test.ParticipationTag).
		LatestMilestoneAsParents().
		FromWallet(env.Wallet1).
		ToWallet(env.Wallet1).
		Amount(env.Wallet1.Balance()).
		TagData([]byte{0x00}).
		Build().
		Store().
		BookOnWallets()

	env.IssueMilestone(invalidParticipation.StoredMessageID()) // 9

	var trackedVote *participation.TrackedParticipation
	trackedVote, err = env.ParticipationManager().ParticipationForOutputID(eventID, castVote.Message().GeneratedUTXO().OutputID())
	require.NoError(t, err)
	require.Equal(t, castVote.Message().StoredMessageID(), trackedVote.MessageID)
	require.Equal(t, milestone.Index(6), trackedVote.StartIndex)
	require.Equal(t, milestone.Index(9), trackedVote.EndIndex)

	env.AssertDefaultBallotAnswerStatus(eventID, 0, 1_000)

}

func TestBallotVoteCancel(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 5)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	// Event should not be accepting votes yet
	require.Equal(t, 0, len(env.ParticipationManager().EventsAcceptingParticipation()))

	env.IssueMilestone() // 5

	// Issue a vote and milestone
	castVote1 := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 6

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 0)

	// Cancel vote
	cancelVote1Msg := env.CancelParticipations(env.Wallet1)
	env.IssueMilestone(append(env.LastMilestoneParents(), cancelVote1Msg.StoredMessageID())...) // 7

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(eventID, 0, 0)

	// Participation again
	castVote2 := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 8

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 1_000)

	// Cancel vote
	cancelVote2Msg := env.CancelParticipations(env.Wallet1)
	env.IssueMilestone(append(env.LastMilestoneParents(), cancelVote2Msg.StoredMessageID())...) // 9

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(eventID, 0, 1_000)

	env.IssueMilestone() // 10

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(eventID, 0, 1_000)

	// Participation again
	castVote3 := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 11

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 2_000)

	env.AssertEventParticipationStatus(eventID, 1, 2)

	// Verify the last issued vote is still active, i.e. EndIndex == 0
	env.AssertTrackedParticipation(eventID, castVote3, 11, 0, 1_000_000)

	// Issue final milestone that ends the participation
	env.IssueMilestone() // 12

	// There should be no active votes left
	env.AssertEventParticipationStatus(eventID, 0, 3)
	env.AssertDefaultBallotAnswerStatus(eventID, 1_000, 3_000)

	// Verify the vote history after the participation ended
	env.AssertTrackedParticipation(eventID, castVote1, 6, 7, 1_000_000)
	env.AssertTrackedParticipation(eventID, castVote2, 8, 9, 1_000_000)
	env.AssertTrackedParticipation(eventID, castVote3, 11, 13, 1_000_000)
}

func TestBallotAddVoteBalanceBySweeping(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 5)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	// Issue a vote and milestone
	castVote1 := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1, 5_000_000) // 8
	require.NotNil(t, castVote1)

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(eventID, 5_000, 5_000)

	// Send more funds to wallet1
	transfer := env.Transfer(env.Wallet2, env.Wallet1, 1_500_000)
	require.Equal(t, 2, len(env.Wallet1.Outputs()))

	env.IssueMilestone(transfer.StoredMessageID()) // 9

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(eventID, 5_000, 10_000)

	// Sweep all funds
	require.Equal(t, 2, len(env.Wallet1.Outputs()))
	castVote2 := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1, 6_500_000) // 10
	require.Equal(t, 1, len(env.Wallet1.Outputs()))

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 1)
	env.AssertDefaultBallotAnswerStatus(eventID, 6_500, 16_500)

	// Verify both votes
	env.AssertTrackedParticipation(eventID, castVote1, 8, 10, 5_000_000)
	env.AssertTrackedParticipation(eventID, castVote2, 10, 0, 6_500_000)
}

func TestBallotAddVoteBalanceByMultipleOutputs(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 5)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	// Issue a vote and milestone
	castVote1 := env.IssueDefaultBallotVoteAndMilestone(eventID, env.Wallet1) // 8
	require.NotNil(t, castVote1)

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(eventID, 5_000, 5_000)

	// Send more funds to wallet1
	transfer := env.Transfer(env.Wallet2, env.Wallet1, 1_500_000)
	require.Equal(t, 2, len(env.Wallet1.Outputs()))

	env.IssueMilestone(transfer.StoredMessageID()) // 9

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(eventID, 5_000, 10_000)

	// Send a separate vote, without sweeping
	require.Equal(t, 2, len(env.Wallet1.Outputs()))
	castVote2 := env.NewParticipationHelper(env.Wallet1).
		Amount(1_500_000).
		UsingOutput(transfer.GeneratedUTXO()).
		AddDefaultBallotVote(eventID).
		Send()
	env.IssueMilestone(castVote2.Message().StoredMessageID()) // 10
	require.Equal(t, 2, len(env.Wallet1.Outputs()))

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 2, 0)
	env.AssertDefaultBallotAnswerStatus(eventID, 6_500, 16_500)

	// Verify both votes
	env.AssertTrackedParticipation(eventID, castVote1, 8, 0, 5_000_000)
	env.AssertTrackedParticipation(eventID, castVote2, 10, 0, 1_500_000)
}

func TestMultipleBallotVotes(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 5)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	wallet1Vote := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{0},
		}).
		Send()

	wallet2Vote := env.NewParticipationHelper(env.Wallet2).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{10},
		}).
		Send()

	wallet3Vote := env.NewParticipationHelper(env.Wallet3).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{20},
		}).
		Send()

	_, confStats := env.IssueMilestone(wallet1Vote.Message().StoredMessageID(), wallet2Vote.Message().StoredMessageID(), wallet3Vote.Message().StoredMessageID()) // 8
	require.Equal(t, 3+1, confStats.MessagesReferenced)                                                                                                           // 3 + milestone itself
	require.Equal(t, 3, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 3, 0)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 5_000, 5_000, 0, 0)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 150_000, 150_000, 0, 10)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 200_000, 200_000, 0, 20)

	// Verify all votes
	env.AssertTrackedParticipation(eventID, wallet1Vote, 8, 0, 5_000_000)
	env.AssertTrackedParticipation(eventID, wallet2Vote, 8, 0, 150_000_000)
	env.AssertTrackedParticipation(eventID, wallet3Vote, 8, 0, 200_000_000)

	env.IssueMilestone() // 9
	env.IssueMilestone() // 10
	env.IssueMilestone() // 11
	env.IssueMilestone() // 12

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 0, 3)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 5_000, 25_000, 0, 0)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 150_000, 750_000, 0, 10)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 200_000, 1000_000, 0, 20)

	// Verify all votes
	env.AssertTrackedParticipation(eventID, wallet1Vote, 8, 13, 5_000_000)
	env.AssertTrackedParticipation(eventID, wallet2Vote, 8, 13, 150_000_000)
	env.AssertTrackedParticipation(eventID, wallet3Vote, 8, 13, 200_000_000)
}

func TestChangeOpinionMidVote(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID := env.StoreDefaultEvent(5, 2, 5)

	event := env.ParticipationManager().Event(eventID)
	require.NotNil(t, event)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	wallet1Vote1 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{10},
		}).
		Send()

	env.IssueMilestone(wallet1Vote1.Message().StoredMessageID()) // 8

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 0)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 5_000, 5_000, 0, 10)

	// Verify all votes
	env.AssertTrackedParticipation(eventID, wallet1Vote1, 8, 0, 5_000_000)

	// Change opinion

	wallet1Vote2 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet1Vote2.Message().StoredMessageID()) // 9

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 1, 1)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 0, 5_000, 0, 10)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 5_000, 5_000, 0, 20)

	// Verify all votes
	env.AssertTrackedParticipation(eventID, wallet1Vote1, 8, 9, 5_000_000)
	env.AssertTrackedParticipation(eventID, wallet1Vote2, 9, 0, 5_000_000)

	// Cancel vote
	cancel := env.CancelParticipations(env.Wallet1)

	env.IssueMilestone(cancel.StoredMessageID()) // 10

	// Verify current vote status
	env.AssertEventParticipationStatus(eventID, 0, 2)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 0, 5_000, 0, 10)
	env.AssertBallotAnswerStatusAtConfirmedMilestoneIndex(eventID, 0, 5_000, 0, 20)

	// Verify all votes
	env.AssertTrackedParticipation(eventID, wallet1Vote1, 8, 9, 5_000_000)
	env.AssertTrackedParticipation(eventID, wallet1Vote2, 9, 10, 5_000_000)
}

func TestMultipleConcurrentEventsWithBallot(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventID1 := env.StoreDefaultEvent(5, 2, 5)
	eventID2 := env.StoreDefaultEvent(7, 2, 5)

	event1 := env.ParticipationManager().Event(eventID1)
	require.NotNil(t, event1)

	event2 := env.ParticipationManager().Event(eventID2)
	require.NotNil(t, event1)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(5), event1.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event1.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event1.EndMilestoneIndex())

	require.Equal(t, milestone.Index(7), event2.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(9), event2.StartMilestoneIndex())
	require.Equal(t, milestone.Index(14), event2.EndMilestoneIndex())

	env.IssueMilestone() // 5

	env.AssertEventsCount(1, 0)
	env.AssertEventParticipationStatus(eventID1, 0, 0)
	env.AssertEventParticipationStatus(eventID2, 0, 0)

	wallet1Vote1 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		// Participation for the commencing event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{10},
		}).
		// Participation too early for the upcoming event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	wallet2Vote1 := env.NewParticipationHelper(env.Wallet2).
		WholeWalletBalance().
		// Participation for the commencing event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{10},
		}).
		Send()

	env.IssueMilestone(wallet1Vote1.Message().StoredMessageID(), wallet2Vote1.Message().StoredMessageID()) // 6

	env.AssertEventsCount(1, 0)
	env.AssertEventParticipationStatus(eventID1, 2, 0)
	env.AssertEventParticipationStatus(eventID2, 0, 0)

	env.IssueMilestone() // 7

	env.AssertEventsCount(2, 1)
	env.AssertEventParticipationStatus(eventID1, 2, 0)
	env.AssertEventParticipationStatus(eventID2, 0, 0)

	wallet3Vote1 := env.NewParticipationHelper(env.Wallet3).
		WholeWalletBalance().
		// Participation for the commencing event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet3Vote1.Message().StoredMessageID()) // 8

	env.AssertEventsCount(2, 1)
	env.AssertEventParticipationStatus(eventID1, 2, 0)
	env.AssertEventParticipationStatus(eventID2, 1, 0)

	env.IssueMilestone() // 9

	env.AssertEventsCount(2, 2)
	env.AssertEventParticipationStatus(eventID1, 2, 0)
	env.AssertEventParticipationStatus(eventID2, 1, 0)

	env.IssueMilestone() // 10

	wallet1Vote2 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		// Keep Participation for the holding event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{10},
		}).
		// Re-Participation holding event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet1Vote2.Message().StoredMessageID()) // 11

	env.AssertEventsCount(2, 2)
	env.AssertEventParticipationStatus(eventID1, 2, 1)
	env.AssertEventParticipationStatus(eventID2, 2, 0)

	wallet4Vote1 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		// Participation for the holding event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{0},
		}).
		Send()

	env.IssueMilestone(wallet4Vote1.Message().StoredMessageID()) // 12

	env.AssertEventsCount(1, 1)
	env.AssertEventParticipationStatus(eventID1, 0, 4)
	env.AssertEventParticipationStatus(eventID2, 2, 0)

	wallet4Vote2 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		// Participation for the holding event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet4Vote2.Message().StoredMessageID()) // 13

	env.AssertEventsCount(1, 1)
	env.AssertEventParticipationStatus(eventID1, 0, 4)
	env.AssertEventParticipationStatus(eventID2, 3, 0)

	env.IssueMilestone() // 14

	env.AssertEventsCount(0, 0)
	env.AssertEventParticipationStatus(eventID1, 0, 4)
	env.AssertEventParticipationStatus(eventID2, 0, 3)

	// Verify all votes
	env.AssertTrackedParticipation(eventID1, wallet1Vote1, 6, 11, 5_000_000) // Voted 1
	env.AssertInvalidParticipation(eventID2, wallet1Vote1)
	env.AssertTrackedParticipation(eventID1, wallet1Vote2, 11, 13, 5_000_000)   // Voted 1
	env.AssertTrackedParticipation(eventID2, wallet1Vote2, 11, 15, 5_000_000)   // Voted 2
	env.AssertTrackedParticipation(eventID1, wallet2Vote1, 6, 13, 150_000_000)  // Voted 1
	env.AssertTrackedParticipation(eventID2, wallet3Vote1, 8, 15, 200_000_000)  // Voted 2
	env.AssertTrackedParticipation(eventID1, wallet4Vote1, 12, 13, 300_000_000) // Voted 0
	env.AssertTrackedParticipation(eventID2, wallet4Vote2, 13, 15, 300_000_000) // Voted 2

	// Verify end results
	env.AssertBallotAnswerStatus(eventID1, event1.EndMilestoneIndex(), 300_000, 300_000, 0, 0)
	env.AssertBallotAnswerStatus(eventID1, event1.EndMilestoneIndex(), 155_000, 775_000, 0, 10)
	env.AssertBallotAnswerStatus(eventID1, event1.EndMilestoneIndex(), 0, 0, 0, 20)

	env.AssertBallotAnswerStatus(eventID2, event2.EndMilestoneIndex(), 0, 0, 0, 0)
	env.AssertBallotAnswerStatus(eventID2, event2.EndMilestoneIndex(), 0, 0, 0, 10)
	env.AssertBallotAnswerStatus(eventID2, event2.EndMilestoneIndex(), 505_000, 1_620_000, 0, 20)
}

func TestMultipleConcurrentEventsWithBallotCalculatedAfterEventEnded(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	event1 := env.DefaultEvent(5, 2, 5)
	event2 := env.DefaultEvent(7, 2, 5)

	eventID1, err := event1.ID()
	require.NoError(t, err)

	eventID2, err := event2.ID()
	require.NoError(t, err)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(5), event1.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event1.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event1.EndMilestoneIndex())

	require.Equal(t, milestone.Index(7), event2.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(9), event2.StartMilestoneIndex())
	require.Equal(t, milestone.Index(14), event2.EndMilestoneIndex())

	env.IssueMilestone() // 5

	wallet1Vote1 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		// Participation for the commencing event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{10},
		}).
		// Participation too early for the upcoming event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	wallet2Vote1 := env.NewParticipationHelper(env.Wallet2).
		WholeWalletBalance().
		// Participation for the commencing event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{10},
		}).
		Send()

	env.IssueMilestone(wallet1Vote1.Message().StoredMessageID(), wallet2Vote1.Message().StoredMessageID()) // 6
	env.IssueMilestone()                                                                                   // 7

	wallet3Vote1 := env.NewParticipationHelper(env.Wallet3).
		WholeWalletBalance().
		// Participation for the commencing event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet3Vote1.Message().StoredMessageID()) //
	env.IssueMilestone()                                         // 9

	// Add event1 during the event
	_, err = env.ParticipationManager().StoreEvent(event1)
	require.NoError(t, err)

	env.IssueMilestone() // 10

	wallet1Vote2 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		// Keep Participation for the holding event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{10},
		}).
		// Re-Participation holding event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet1Vote2.Message().StoredMessageID()) // 11

	wallet4Vote1 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		// Participation for the holding event1
		AddParticipation(&participation.Participation{
			EventID: eventID1,
			Answers: []byte{0},
		}).
		Send()

	env.IssueMilestone(wallet4Vote1.Message().StoredMessageID()) // 12

	wallet4Vote2 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		// Participation for the holding event2
		AddParticipation(&participation.Participation{
			EventID: eventID2,
			Answers: []byte{20},
		}).
		Send()

	env.IssueMilestone(wallet4Vote2.Message().StoredMessageID()) // 13
	env.IssueMilestone()                                         // 14

	// Add event2 after the event
	_, err = env.ParticipationManager().StoreEvent(event2)
	require.NoError(t, err)

	// Verify all votes
	env.AssertTrackedParticipation(eventID1, wallet1Vote1, 6, 11, 5_000_000) // Voted 1
	env.AssertInvalidParticipation(eventID2, wallet1Vote1)
	env.AssertTrackedParticipation(eventID1, wallet1Vote2, 11, 13, 5_000_000)   // Voted 1
	env.AssertTrackedParticipation(eventID2, wallet1Vote2, 11, 15, 5_000_000)   // Voted 2
	env.AssertTrackedParticipation(eventID1, wallet2Vote1, 6, 13, 150_000_000)  // Voted 1
	env.AssertTrackedParticipation(eventID2, wallet3Vote1, 8, 15, 200_000_000)  // Voted 2
	env.AssertTrackedParticipation(eventID1, wallet4Vote1, 12, 13, 300_000_000) // Voted 0
	env.AssertTrackedParticipation(eventID2, wallet4Vote2, 13, 15, 300_000_000) // Voted 2

	// Verify end results
	env.AssertBallotAnswerStatus(eventID1, event1.EndMilestoneIndex(), 300_000, 300_000, 0, 0)
	env.AssertBallotAnswerStatus(eventID1, event1.EndMilestoneIndex(), 155_000, 775_000, 0, 10)
	env.AssertBallotAnswerStatus(eventID1, event1.EndMilestoneIndex(), 0, 0, 0, 20)

	env.AssertBallotAnswerStatus(eventID2, event2.EndMilestoneIndex(), 0, 0, 0, 0)
	env.AssertBallotAnswerStatus(eventID2, event2.EndMilestoneIndex(), 0, 0, 0, 10)
	env.AssertBallotAnswerStatus(eventID2, event2.EndMilestoneIndex(), 505_000, 1_620_000, 0, 20)
}

func TestStakingRewards(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 1_587_529, 5_589_977, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventBuilder := participation.NewEventBuilder("AlbinoPugCoin", 5, 7, 12, "The first DogCoin on the Tangle")
	eventBuilder.Payload(&participation.Staking{
		Text:                   "The rarest DogCoin on earth",
		Symbol:                 "APUG",
		Numerator:              25,
		Denominator:            100,
		RequiredMinimumRewards: 2_000_000,
		AdditionalInfo:         "Have you seen an albino Pug?",
	})

	event, err := eventBuilder.Build()
	require.NoError(t, err)

	eventID, err := env.ParticipationManager().StoreEvent(event)
	require.NoError(t, err)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.AssertEventsCount(1, 0)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 0, 0)

	stakeWallet1 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	stakeWallet2 := env.NewParticipationHelper(env.Wallet2).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	stakeWallet3 := env.NewParticipationHelper(env.Wallet3).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	preStakeWallet4 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	env.IssueMilestone(stakeWallet1.Message().StoredMessageID(), stakeWallet2.Message().StoredMessageID(), stakeWallet3.Message().StoredMessageID(), preStakeWallet4.Message().StoredMessageID()) // 6
	env.AssertEventsCount(1, 0)
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 0)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 0)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 0)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 0)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977+300_000_000, 0)

	cancelPreStakeWallet4 := env.CancelParticipations(env.Wallet4)

	env.IssueMilestone(cancelPreStakeWallet4.StoredMessageID()) // 7
	env.AssertEventsCount(1, 1)
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 0)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 0)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 0)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 0)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977, 0)

	env.IssueMilestone() // 8
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 1_250_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 396_882)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 1_397_494)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 0)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977, 1_250_000+396_882+1_397_494)

	stakeWallet4 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	env.IssueMilestone(stakeWallet4.Message().StoredMessageID()) // 9
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 2_500_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 793_764)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 2_794_988)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 75_000_000)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977+300_000_000, 2_500_000+793_764+2_794_988+75_000_000)

	cancelStakeWallet4 := env.CancelParticipations(env.Wallet4)

	env.IssueMilestone(cancelStakeWallet4.StoredMessageID()) // 10
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 3_750_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 1_190_646)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 4_192_482)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 75_000_000)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977, 3_750_000+1_190_646+4_192_482+75_000_000)

	env.IssueMilestone() // 11
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 5_000_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 1_587_528)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 5_589_976)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 75_000_000)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977, 5_000_000+1_587_528+5_589_976+75_000_000)

	env.IssueMilestone() // 12
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 6_250_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 1_984_410)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 6_987_470)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 75_000_000)

	env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(eventID, 5_000_000+1_587_529+5_589_977, 6_250_000+1_984_410+6_987_470+75_000_000)

	env.IssueMilestone() // 13
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 6_250_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 1_984_410)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 6_987_470)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 75_000_000)

	env.AssertStakingRewardsStatus(eventID, 12, 5_000_000+1_587_529+5_589_977, 6_250_000+1_984_410+6_987_470+75_000_000)

	totalRewards := uint64(0)
	addresses := make(map[string]uint64)
	env.ParticipationManager().ForEachAddressStakingParticipation(eventID, env.ConfirmedMilestoneIndex(), func(address iotago.Address, _ *participation.TrackedParticipation, rewards uint64) bool {
		totalRewards += rewards
		addresses[address.String()] += rewards
		return true
	})

	// Filter out minimum rewards
	for addr, amount := range addresses {
		if amount < event.Staking().RequiredMinimumRewards {
			totalRewards -= amount
			delete(addresses, addr)
		}
	}

	require.Exactly(t, totalRewards, uint64(6_250_000+6_987_470+75_000_000))
	_, wallet1Found := addresses[env.Wallet1.Address().String()]
	_, wallet2Found := addresses[env.Wallet2.Address().String()]
	_, wallet3Found := addresses[env.Wallet3.Address().String()]
	_, wallet4Found := addresses[env.Wallet4.Address().String()]
	require.True(t, wallet1Found)
	require.False(t, wallet2Found)
	require.True(t, wallet3Found)
	require.True(t, wallet4Found)

	totalRewardsWithoutFilter := uint64(0)
	env.ParticipationManager().ForEachAddressStakingParticipation(eventID, env.ConfirmedMilestoneIndex(), func(address iotago.Address, _ *participation.TrackedParticipation, rewards uint64) bool {
		totalRewardsWithoutFilter += rewards
		return true
	})
	require.Exactly(t, totalRewardsWithoutFilter, uint64(6_250_000+1_984_410+6_987_470+75_000_000))
}

func TestStakingRewardsCalculatedAfterEventEnded(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 1_587_529, 5_589_977, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventBuilder := participation.NewEventBuilder("AlbinoPugCoin", 5, 7, 12, "The first DogCoin on the Tangle")
	eventBuilder.Payload(&participation.Staking{
		Text:           "The rarest DogCoin on earth",
		Symbol:         "APUG",
		Numerator:      25,
		Denominator:    100,
		AdditionalInfo: "Have you seen an albino Pug?",
	})

	event, err := eventBuilder.Build()
	require.NoError(t, err)

	eventID, err := event.ID()
	require.NoError(t, err)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5

	stakeWallet1 := env.NewParticipationHelper(env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	stakeWallet2 := env.NewParticipationHelper(env.Wallet2).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	stakeWallet3 := env.NewParticipationHelper(env.Wallet3).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	env.IssueMilestone(stakeWallet1.Message().StoredMessageID(), stakeWallet2.Message().StoredMessageID(), stakeWallet3.Message().StoredMessageID()) // 6
	env.IssueMilestone()                                                                                                                             // 7
	env.IssueMilestone()                                                                                                                             // 8

	stakeWallet4 := env.NewParticipationHelper(env.Wallet4).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: eventID,
			Answers: []byte{},
		}).
		Send()

	env.IssueMilestone(stakeWallet4.Message().StoredMessageID()) // 9

	cancelStakeWallet4 := env.CancelParticipations(env.Wallet4)

	env.IssueMilestone(cancelStakeWallet4.StoredMessageID()) // 10
	env.IssueMilestone()                                     // 11
	env.IssueMilestone()                                     // 12
	env.IssueMilestone()                                     // 13

	// Event is already in the past, but we can still add it
	_, err = env.ParticipationManager().StoreEvent(event)
	require.NoError(t, err)

	env.AssertStakingRewardsStatus(eventID, 5, 0, 0)
	env.AssertStakingRewardsStatus(eventID, 6, 5_000_000+1_587_529+5_589_977, 0)
	env.AssertStakingRewardsStatus(eventID, 8, 5_000_000+1_587_529+5_589_977, 1_250_000+396_882+1_397_494)
	env.AssertStakingRewardsStatus(eventID, 9, 5_000_000+1_587_529+5_589_977+300_000_000, 2_500_000+793_764+2_794_988+75_000_000)
	env.AssertStakingRewardsStatus(eventID, 10, 5_000_000+1_587_529+5_589_977, 3_750_000+1_190_646+4_192_482+75_000_000)
	env.AssertStakingRewardsStatus(eventID, 11, 5_000_000+1_587_529+5_589_977, 5_000_000+1_587_528+5_589_976+75_000_000)
	env.AssertStakingRewardsStatus(eventID, 12, 5_000_000+1_587_529+5_589_977, 6_250_000+1_984_410+6_987_470+75_000_000)
	env.AssertRewardBalance(eventID, env.Wallet1.Address(), 6_250_000)
	env.AssertRewardBalance(eventID, env.Wallet2.Address(), 1_984_410)
	env.AssertRewardBalance(eventID, env.Wallet3.Address(), 6_987_470)
	env.AssertRewardBalance(eventID, env.Wallet4.Address(), 75_000_000)
}

func TestMultipleParticipationsAreNotCounted(t *testing.T) {

	env := test.NewParticipationTestEnv(t, 5_000_000, 1_587_529, 5_589_977, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventBuilder := participation.NewEventBuilder("AlbinoPugCoin", 5, 7, 12, "The first DogCoin on the Tangle")
	eventBuilder.Payload(&participation.Staking{
		Text:                   "The rarest DogCoin on earth",
		Symbol:                 "APUG",
		Numerator:              25,
		Denominator:            100,
		RequiredMinimumRewards: 0,
		AdditionalInfo:         "Have you seen an albino Pug?",
	})

	event, err := eventBuilder.Build()
	require.NoError(t, err)

	eventID, err := env.ParticipationManager().StoreEvent(event)
	require.NoError(t, err)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(5), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), event.EndMilestoneIndex())

	env.IssueMilestone() // 5

	// Forcedly craft a taggedData that participates twice in the same tag
	ms := marshalutil.New()
	ms.WriteUint8(2)
	ms.WriteBytes(eventID[:])
	ms.WriteUint8(0)
	ms.WriteBytes(eventID[:])
	ms.WriteUint8(0)

	doubleStakeWallet1 := env.NewMessageBuilder(test.ParticipationTag).
		LatestMilestoneAsParents().
		FromWallet(env.Wallet1).
		ToWallet(env.Wallet1).
		Amount(env.Wallet1.Balance()).
		TagData(ms.Bytes()).
		Build().
		Store().
		BookOnWallets()

	env.IssueMilestone(doubleStakeWallet1.StoredMessageID()) // 6

	_, err = env.ParticipationManager().ParticipationForOutputID(eventID, doubleStakeWallet1.GeneratedUTXO().OutputID())
	require.Error(t, err)
	require.ErrorIs(t, err, participation.ErrUnknownParticipation)
}

func TestStoreEventCanOverflow(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 1_587_529, 5_589_977, 300_000_000, false)
	defer env.Cleanup()

	_, err := env.ParticipationManager().StoreEvent(RandStakingEvent(6_636, 1, 1))
	require.NoError(t, err)

	_, err = env.ParticipationManager().StoreEvent(RandStakingEvent(6_637, 1, 1))
	require.ErrorIs(t, err, participation.ErrParticipationEventStakingCanOverflow)
}
