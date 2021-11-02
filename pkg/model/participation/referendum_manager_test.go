package participation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/gohornet/hornet/pkg/model/participation/test"
)

func TestReferendumStateHelpers(t *testing.T) {

	referendumStartIndex := milestone.Index(90)
	referendumStartHoldingIndex := milestone.Index(100)
	referendumEndIndex := milestone.Index(200)

	referendumBuilder := participation.NewParticipationEventBuilder("Test", referendumStartIndex, referendumStartHoldingIndex, referendumEndIndex, "Sample")

	questionBuilder := participation.NewQuestionBuilder("Q1", "-")
	questionBuilder.AddAnswer(&participation.Answer{
		Index:          1,
		Text:           "A1",
		AdditionalInfo: "-",
	})
	questionBuilder.AddAnswer(&participation.Answer{
		Index:          2,
		Text:           "A2",
		AdditionalInfo: "-",
	})

	question, err := questionBuilder.Build()
	require.NoError(t, err)

	questionsBuilder := participation.NewBallotBuilder()
	questionsBuilder.AddQuestion(question)
	payload, err := questionsBuilder.Build()
	require.NoError(t, err)

	referendumBuilder.Payload(payload)

	ref, err := referendumBuilder.Build()
	require.NoError(t, err)

	// Verify status
	require.Equal(t, "upcoming", ref.Status(89))
	require.Equal(t, "commencing", ref.Status(90))
	require.Equal(t, "commencing", ref.Status(91))
	require.Equal(t, "holding", ref.Status(100))
	require.Equal(t, "holding", ref.Status(101))
	require.Equal(t, "holding", ref.Status(199))
	require.Equal(t, "ended", ref.Status(200))
	require.Equal(t, "ended", ref.Status(201))

	// Verify ReferendumIsAcceptingVotes
	require.False(t, ref.IsAcceptingParticipation(89))
	require.True(t, ref.IsAcceptingParticipation(90))
	require.True(t, ref.IsAcceptingParticipation(91))
	require.True(t, ref.IsAcceptingParticipation(99))
	require.True(t, ref.IsAcceptingParticipation(100))
	require.True(t, ref.IsAcceptingParticipation(101))
	require.True(t, ref.IsAcceptingParticipation(199))
	require.False(t, ref.IsAcceptingParticipation(200))
	require.False(t, ref.IsAcceptingParticipation(201))

	// Verify ReferendumIsCountingVotes
	require.False(t, ref.IsCountingParticipation(89))
	require.False(t, ref.IsCountingParticipation(90))
	require.False(t, ref.IsCountingParticipation(91))
	require.False(t, ref.IsCountingParticipation(99))
	require.True(t, ref.IsCountingParticipation(100))
	require.True(t, ref.IsCountingParticipation(101))
	require.True(t, ref.IsCountingParticipation(199))
	require.False(t, ref.IsCountingParticipation(200))
	require.False(t, ref.IsCountingParticipation(201))

	// Verify ReferendumShouldAcceptVotes
	require.False(t, ref.ShouldAcceptParticipation(89))
	require.False(t, ref.ShouldAcceptParticipation(90))
	require.True(t, ref.ShouldAcceptParticipation(91))
	require.True(t, ref.ShouldAcceptParticipation(99))
	require.True(t, ref.ShouldAcceptParticipation(100))
	require.True(t, ref.ShouldAcceptParticipation(101))
	require.True(t, ref.ShouldAcceptParticipation(199))
	require.True(t, ref.ShouldAcceptParticipation(200))
	require.False(t, ref.ShouldAcceptParticipation(201))

	// Verify ReferendumShouldCountVotes
	require.False(t, ref.ShouldCountParticipation(89))
	require.False(t, ref.ShouldCountParticipation(90))
	require.False(t, ref.ShouldCountParticipation(91))
	require.False(t, ref.ShouldCountParticipation(99))
	require.False(t, ref.ShouldCountParticipation(100))
	require.True(t, ref.ShouldCountParticipation(101))
	require.True(t, ref.ShouldCountParticipation(199))
	require.True(t, ref.ShouldCountParticipation(200))
	require.False(t, ref.ShouldCountParticipation(201))
}

func TestReferendumStates(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	require.Empty(t, env.ReferendumManager().ParticipationEvents())
	referendumID := env.RegisterDefaultReferendum(5, 1, 2)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(6), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(8), ref.EndMilestoneIndex())

	// No participation should be running right now
	require.Equal(t, 1, len(env.ReferendumManager().ParticipationEvents()))
	env.AssertReferendumsCount(0, 0)

	env.IssueMilestone() // 5

	// ParticipationEvent should be accepting votes, but not counting
	require.Equal(t, 1, len(env.ReferendumManager().ParticipationEvents()))
	env.AssertReferendumsCount(1, 0)

	env.IssueMilestone() // 6

	// ParticipationEvent should be accepting and counting votes
	require.Equal(t, 1, len(env.ReferendumManager().ParticipationEvents()))
	env.AssertReferendumsCount(1, 1)

	env.IssueMilestone() // 7

	// ParticipationEvent should be ended
	require.Equal(t, 1, len(env.ReferendumManager().ParticipationEvents()))
	env.AssertReferendumsCount(1, 1)

	env.IssueMilestone() // 8

	// ParticipationEvent should be ended
	require.Equal(t, 1, len(env.ReferendumManager().ParticipationEvents()))
	env.AssertReferendumsCount(0, 0)
}

func TestSingleReferendumVote(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 3)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(10), ref.EndMilestoneIndex())

	// ParticipationEvent should not be accepting votes yet
	require.Equal(t, 0, len(env.ReferendumManager().EventsAcceptingParticipation()))

	// Issue a vote and milestone
	env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 5

	// Votes should not have been counted so far because it was not accepting votes yet
	status, err := env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	// ParticipationEvent should be accepting votes now
	require.Equal(t, 1, len(env.ReferendumManager().EventsAcceptingParticipation()))

	// Vote again
	castVote := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 6

	// ParticipationEvent should be accepting votes, but the vote should not be weighted yet, just added to the current status
	env.AssertReferendumsCount(1, 0)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 7

	// ParticipationEvent should be accepting and counting votes, but the vote we did before should not be weighted yet
	env.AssertReferendumsCount(1, 1)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 8

	// ParticipationEvent should be accepting and counting votes, the vote should now be weighted
	env.AssertReferendumsCount(1, 1)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 9

	// ParticipationEvent should be accepting and counting votes, the vote should now be weighted double
	env.AssertReferendumsCount(1, 1)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Equal(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Equal(t, uint64(2_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 10

	// ParticipationEvent should be ended
	env.AssertReferendumsCount(0, 0)
	env.AssertDefaultBallotAnswerStatus(referendumID, 1_000_000, 3_000_000)

	env.IssueMilestone() // 11

	trackedVote, err := env.ReferendumManager().VoteForOutputID(referendumID, castVote.Message().GeneratedUTXO().OutputID())
	require.Equal(t, castVote.Message().StoredMessageID(), trackedVote.MessageID)
	require.Equal(t, milestone.Index(6), trackedVote.StartIndex)
	require.Equal(t, milestone.Index(10), trackedVote.EndIndex)

	messageFromReferendum, err := env.ReferendumManager().MessageForMessageID(trackedVote.MessageID)
	require.NoError(t, err)
	require.NotNil(t, messageFromReferendum)
	require.Equal(t, messageFromReferendum.Message(), castVote.Message().IotaMessage())
}

func TestReferendumVoteCancel(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref.EndMilestoneIndex())

	// ParticipationEvent should not be accepting votes yet
	require.Equal(t, 0, len(env.ReferendumManager().EventsAcceptingParticipation()))

	env.IssueMilestone() // 5

	// Issue a vote and milestone
	castVote1 := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 6

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(referendumID, 1_000_000, 0)

	// Cancel vote
	cancelVote1Msg := env.CancelVote(env.Wallet1)
	env.IssueMilestone(cancelVote1Msg.StoredMessageID(), env.LastMilestoneMessageID()) // 7

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(referendumID, 0, 0)

	// Vote again
	castVote2 := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 8

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(referendumID, 1_000_000, 1_000_000)

	// Cancel vote
	cancelVote2Msg := env.CancelVote(env.Wallet1)
	env.IssueMilestone(cancelVote2Msg.StoredMessageID(), env.LastMilestoneMessageID()) // 9

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(referendumID, 0, 1_000_000)

	env.IssueMilestone() // 10

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(referendumID, 0, 1_000_000)

	// Vote again
	castVote3 := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 11

	// Verify vote
	env.AssertDefaultBallotAnswerStatus(referendumID, 1_000_000, 2_000_000)

	env.AssertReferendumVoteStatus(referendumID, 1, 2)

	// Verify the last issued vote is still active, i.e. EndIndex == 0
	env.AssertTrackedVote(referendumID, castVote3, 11, 0, 1_000_000)

	// Issue final milestone that ends the participation
	env.IssueMilestone() // 12

	// There should be no active votes left
	env.AssertReferendumVoteStatus(referendumID, 0, 3)
	env.AssertDefaultBallotAnswerStatus(referendumID, 1_000_000, 3_000_000)

	// Verify the vote history after the participation ended
	env.AssertTrackedVote(referendumID, castVote1, 6, 7, 1_000_000)
	env.AssertTrackedVote(referendumID, castVote2, 8, 9, 1_000_000)
	env.AssertTrackedVote(referendumID, castVote3, 11, 12, 1_000_000)
}

func TestReferendumVoteAddVoteBalanceBySweeping(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	// Issue a vote and milestone
	castVote1 := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1, 5_000_000) // 8
	require.NotNil(t, castVote1)

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(referendumID, 5_000_000, 5_000_000)

	// Send more funds to wallet1
	transfer := env.Transfer(env.Wallet2, env.Wallet1, 1_500_000)
	require.Equal(t, 2, len(env.Wallet1.Outputs()))

	env.IssueMilestone(transfer.StoredMessageID()) // 9

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(referendumID, 5_000_000, 10_000_000)

	// Sweep all funds
	require.Equal(t, 2, len(env.Wallet1.Outputs()))
	castVote2 := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1, 6_500_000) // 10
	require.Equal(t, 1, len(env.Wallet1.Outputs()))

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 1)
	env.AssertDefaultBallotAnswerStatus(referendumID, 6_500_000, 16_500_000)

	// Verify both votes
	env.AssertTrackedVote(referendumID, castVote1, 8, 10, 5_000_000)
	env.AssertTrackedVote(referendumID, castVote2, 10, 0, 6_500_000)
}

func TestReferendumVoteAddVoteBalanceByMultipleOutputs(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	// Issue a vote and milestone
	castVote1 := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 8
	require.NotNil(t, castVote1)

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(referendumID, 5_000_000, 5_000_000)

	// Send more funds to wallet1
	transfer := env.Transfer(env.Wallet2, env.Wallet1, 1_500_000)
	require.Equal(t, 2, len(env.Wallet1.Outputs()))

	env.IssueMilestone(transfer.StoredMessageID()) // 9

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 0)
	env.AssertDefaultBallotAnswerStatus(referendumID, 5_000_000, 10_000_000)

	// Cast a separate vote, without sweeping
	require.Equal(t, 2, len(env.Wallet1.Outputs()))
	castVote2 := env.NewVoteBuilder(env.Wallet1).
		Amount(1_500_000).
		UsingOutput(transfer.GeneratedUTXO()).
		AddDefaultVote(referendumID).
		Cast()
	env.IssueMilestone(castVote2.Message().StoredMessageID()) // 10
	require.Equal(t, 2, len(env.Wallet1.Outputs()))

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 2, 0)
	env.AssertDefaultBallotAnswerStatus(referendumID, 6_500_000, 16_500_000)

	// Verify both votes
	env.AssertTrackedVote(referendumID, castVote1, 8, 0, 5_000_000)
	env.AssertTrackedVote(referendumID, castVote2, 10, 0, 1_500_000)
}

func TestReferendumMultipleVotes(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	wallet1Vote := env.NewVoteBuilder(env.Wallet1).
		WholeWalletBalance().
		AddVote(&participation.Vote{
			ReferendumID: referendumID,
			Answers:      []byte{0},
		}).
		Cast()

	wallet2Vote := env.NewVoteBuilder(env.Wallet2).
		WholeWalletBalance().
		AddVote(&participation.Vote{
			ReferendumID: referendumID,
			Answers:      []byte{1},
		}).
		Cast()

	wallet3Vote := env.NewVoteBuilder(env.Wallet3).
		WholeWalletBalance().
		AddVote(&participation.Vote{
			ReferendumID: referendumID,
			Answers:      []byte{2},
		}).
		Cast()

	_, confStats := env.IssueMilestone(wallet1Vote.Message().StoredMessageID(), wallet2Vote.Message().StoredMessageID(), wallet3Vote.Message().StoredMessageID()) // 8
	require.Equal(t, 3+1, confStats.MessagesReferenced)                                                                                                           // 3 + milestone itself
	require.Equal(t, 3, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 3, 0)
	env.AssertBallotAnswerStatus(referendumID, 5_000_000, 5_000_000, 0, 0)
	env.AssertBallotAnswerStatus(referendumID, 150_000_000, 150_000_000, 0, 1)
	env.AssertBallotAnswerStatus(referendumID, 200_000_000, 200_000_000, 0, 2)

	// Verify all votes
	env.AssertTrackedVote(referendumID, wallet1Vote, 8, 0, 5_000_000)
	env.AssertTrackedVote(referendumID, wallet2Vote, 8, 0, 150_000_000)
	env.AssertTrackedVote(referendumID, wallet3Vote, 8, 0, 200_000_000)

	env.IssueMilestone() // 9
	env.IssueMilestone() // 10
	env.IssueMilestone() // 11
	env.IssueMilestone() // 12

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 0, 3)
	env.AssertBallotAnswerStatus(referendumID, 5_000_000, 25_000_000, 0, 0)
	env.AssertBallotAnswerStatus(referendumID, 150_000_000, 750_000_000, 0, 1)
	env.AssertBallotAnswerStatus(referendumID, 200_000_000, 1000_000_000, 0, 2)

	// Verify all votes
	env.AssertTrackedVote(referendumID, wallet1Vote, 8, 12, 5_000_000)
	env.AssertTrackedVote(referendumID, wallet2Vote, 8, 12, 150_000_000)
	env.AssertTrackedVote(referendumID, wallet3Vote, 8, 12, 200_000_000)
}

func TestReferendumChangeOpinionMidVote(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref.EndMilestoneIndex())

	env.IssueMilestone() // 5
	env.IssueMilestone() // 6
	env.IssueMilestone() // 7

	wallet1Vote1 := env.NewVoteBuilder(env.Wallet1).
		WholeWalletBalance().
		AddVote(&participation.Vote{
			ReferendumID: referendumID,
			Answers:      []byte{1},
		}).
		Cast()

	env.IssueMilestone(wallet1Vote1.Message().StoredMessageID()) // 8

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 0)
	env.AssertBallotAnswerStatus(referendumID, 5_000_000, 5_000_000, 0, 1)

	// Verify all votes
	env.AssertTrackedVote(referendumID, wallet1Vote1, 8, 0, 5_000_000)

	// Change opinion

	wallet1Vote2 := env.NewVoteBuilder(env.Wallet1).
		WholeWalletBalance().
		AddVote(&participation.Vote{
			ReferendumID: referendumID,
			Answers:      []byte{2},
		}).
		Cast()

	env.IssueMilestone(wallet1Vote2.Message().StoredMessageID()) // 9

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 1, 1)
	env.AssertBallotAnswerStatus(referendumID, 0, 5_000_000, 0, 1)
	env.AssertBallotAnswerStatus(referendumID, 5_000_000, 5_000_000, 0, 2)

	// Verify all votes
	env.AssertTrackedVote(referendumID, wallet1Vote1, 8, 9, 5_000_000)
	env.AssertTrackedVote(referendumID, wallet1Vote2, 9, 0, 5_000_000)

	// Cancel vote
	cancel := env.CancelVote(env.Wallet1)

	env.IssueMilestone(cancel.StoredMessageID()) // 10

	// Verify current vote status
	env.AssertReferendumVoteStatus(referendumID, 0, 2)
	env.AssertBallotAnswerStatus(referendumID, 0, 5_000_000, 0, 1)
	env.AssertBallotAnswerStatus(referendumID, 0, 5_000_000, 0, 2)

	// Verify all votes
	env.AssertTrackedVote(referendumID, wallet1Vote1, 8, 9, 5_000_000)
	env.AssertTrackedVote(referendumID, wallet1Vote2, 9, 10, 5_000_000)
}

func TestReferendumMultipleConcurrentReferendums(t *testing.T) {
	env := test.NewParticipationTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID1 := env.RegisterDefaultReferendum(5, 2, 5)
	referendumID2 := env.RegisterDefaultReferendum(7, 2, 5)

	ref1 := env.ReferendumManager().Referendum(referendumID1)
	require.NotNil(t, ref1)

	ref2 := env.ReferendumManager().Referendum(referendumID2)
	require.NotNil(t, ref1)

	// Verify the configured participation indexes
	require.Equal(t, milestone.Index(5), ref1.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref1.StartMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref1.EndMilestoneIndex())

	require.Equal(t, milestone.Index(7), ref2.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(9), ref2.StartMilestoneIndex())
	require.Equal(t, milestone.Index(14), ref2.EndMilestoneIndex())

	env.IssueMilestone() // 5

	env.AssertReferendumsCount(1, 0)
	env.AssertReferendumVoteStatus(referendumID1, 0, 0)
	env.AssertReferendumVoteStatus(referendumID2, 0, 0)

	wallet1Vote1 := env.NewVoteBuilder(env.Wallet1).
		WholeWalletBalance().
		// Vote for the commencing ref1
		AddVote(&participation.Vote{
			ReferendumID: referendumID1,
			Answers:      []byte{1},
		}).
		// Vote too early for the upcoming ref2
		AddVote(&participation.Vote{
			ReferendumID: referendumID2,
			Answers:      []byte{2},
		}).
		Cast()

	wallet2Vote1 := env.NewVoteBuilder(env.Wallet2).
		WholeWalletBalance().
		// Vote for the commencing ref1
		AddVote(&participation.Vote{
			ReferendumID: referendumID1,
			Answers:      []byte{1},
		}).
		Cast()

	env.IssueMilestone(wallet1Vote1.Message().StoredMessageID(), wallet2Vote1.Message().StoredMessageID()) // 6

	env.AssertReferendumsCount(1, 0)
	env.AssertReferendumVoteStatus(referendumID1, 2, 0)
	env.AssertReferendumVoteStatus(referendumID2, 0, 0)

	env.IssueMilestone() // 7

	env.AssertReferendumsCount(2, 1)
	env.AssertReferendumVoteStatus(referendumID1, 2, 0)
	env.AssertReferendumVoteStatus(referendumID2, 0, 0)

	wallet3Vote1 := env.NewVoteBuilder(env.Wallet3).
		WholeWalletBalance().
		// Vote for the commencing ref2
		AddVote(&participation.Vote{
			ReferendumID: referendumID2,
			Answers:      []byte{2},
		}).
		Cast()

	env.IssueMilestone(wallet3Vote1.Message().StoredMessageID()) // 8

	env.AssertReferendumsCount(2, 1)
	env.AssertReferendumVoteStatus(referendumID1, 2, 0)
	env.AssertReferendumVoteStatus(referendumID2, 1, 0)

	env.IssueMilestone() // 9

	env.AssertReferendumsCount(2, 2)
	env.AssertReferendumVoteStatus(referendumID1, 2, 0)
	env.AssertReferendumVoteStatus(referendumID2, 1, 0)

	env.IssueMilestone() // 10

	wallet1Vote2 := env.NewVoteBuilder(env.Wallet1).
		WholeWalletBalance().
		// Keep Vote for the holding ref1
		AddVote(&participation.Vote{
			ReferendumID: referendumID1,
			Answers:      []byte{1},
		}).
		// Re-Vote holding ref2
		AddVote(&participation.Vote{
			ReferendumID: referendumID2,
			Answers:      []byte{2},
		}).
		Cast()

	env.IssueMilestone(wallet1Vote2.Message().StoredMessageID()) // 11

	env.AssertReferendumsCount(2, 2)
	env.AssertReferendumVoteStatus(referendumID1, 2, 1)
	env.AssertReferendumVoteStatus(referendumID2, 2, 0)

	wallet4Vote1 := env.NewVoteBuilder(env.Wallet4).
		WholeWalletBalance().
		// Vote for the holding ref1
		AddVote(&participation.Vote{
			ReferendumID: referendumID1,
			Answers:      []byte{0},
		}).
		Cast()

	env.IssueMilestone(wallet4Vote1.Message().StoredMessageID()) // 12

	env.AssertReferendumsCount(1, 1)
	env.AssertReferendumVoteStatus(referendumID1, 0, 4)
	env.AssertReferendumVoteStatus(referendumID2, 2, 0)

	wallet4Vote2 := env.NewVoteBuilder(env.Wallet4).
		WholeWalletBalance().
		// Vote for the holding ref2
		AddVote(&participation.Vote{
			ReferendumID: referendumID2,
			Answers:      []byte{2},
		}).
		Cast()

	env.IssueMilestone(wallet4Vote2.Message().StoredMessageID()) // 13

	env.AssertReferendumsCount(1, 1)
	env.AssertReferendumVoteStatus(referendumID1, 0, 4)
	env.AssertReferendumVoteStatus(referendumID2, 3, 0)

	env.IssueMilestone() // 14

	env.AssertReferendumsCount(0, 0)
	env.AssertReferendumVoteStatus(referendumID1, 0, 4)
	env.AssertReferendumVoteStatus(referendumID2, 0, 3)

	// Verify all votes
	env.AssertTrackedVote(referendumID1, wallet1Vote1, 6, 11, 5_000_000) // Voted 1
	env.AssertInvalidVote(referendumID2, wallet1Vote1)
	env.AssertTrackedVote(referendumID1, wallet1Vote2, 11, 12, 5_000_000)   // Voted 1
	env.AssertTrackedVote(referendumID2, wallet1Vote2, 11, 14, 5_000_000)   // Voted 2
	env.AssertTrackedVote(referendumID1, wallet2Vote1, 6, 12, 150_000_000)  // Voted 1
	env.AssertTrackedVote(referendumID2, wallet3Vote1, 8, 14, 200_000_000)  // Voted 2
	env.AssertTrackedVote(referendumID1, wallet4Vote1, 12, 12, 300_000_000) // Voted 0
	env.AssertTrackedVote(referendumID2, wallet4Vote2, 13, 14, 300_000_000) // Voted 2

	// Verify end results
	env.AssertBallotAnswerStatus(referendumID1, 300_000_000, 300_000_000, 0, 0)
	env.AssertBallotAnswerStatus(referendumID1, 155_000_000, 775_000_000, 0, 1)
	env.AssertBallotAnswerStatus(referendumID1, 0, 0, 0, 2)

	env.AssertBallotAnswerStatus(referendumID2, 0, 0, 0, 0)
	env.AssertBallotAnswerStatus(referendumID2, 0, 0, 0, 1)
	env.AssertBallotAnswerStatus(referendumID2, 505_000_000, 1_620_000_000, 0, 2)
}
