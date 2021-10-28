package referendum_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/referendum"
	"github.com/gohornet/hornet/pkg/model/referendum/test"
)

func TestReferendumStateHelpers(t *testing.T) {

	referendumStartIndex := milestone.Index(90)
	referendumStartHoldingIndex := milestone.Index(100)
	referendumEndIndex := milestone.Index(200)

	referendumBuilder := referendum.NewReferendumBuilder("Test", referendumStartIndex, referendumStartHoldingIndex, referendumEndIndex, "Sample")

	questionBuilder := referendum.NewQuestionBuilder("Q1", "-")
	questionBuilder.AddAnswer(&referendum.Answer{
		Index:          1,
		Text:           "A1",
		AdditionalInfo: "-",
	})
	questionBuilder.AddAnswer(&referendum.Answer{
		Index:          2,
		Text:           "A2",
		AdditionalInfo: "-",
	})

	question, err := questionBuilder.Build()
	require.NoError(t, err)

	questionsBuilder := referendum.NewBallotBuilder()
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
	require.False(t, ref.IsAcceptingVotes(89))
	require.True(t, ref.IsAcceptingVotes(90))
	require.True(t, ref.IsAcceptingVotes(91))
	require.True(t, ref.IsAcceptingVotes(99))
	require.True(t, ref.IsAcceptingVotes(100))
	require.True(t, ref.IsAcceptingVotes(101))
	require.True(t, ref.IsAcceptingVotes(199))
	require.False(t, ref.IsAcceptingVotes(200))
	require.False(t, ref.IsAcceptingVotes(201))

	// Verify ReferendumIsCountingVotes
	require.False(t, ref.IsCountingVotes(89))
	require.False(t, ref.IsCountingVotes(90))
	require.False(t, ref.IsCountingVotes(91))
	require.False(t, ref.IsCountingVotes(99))
	require.True(t, ref.IsCountingVotes(100))
	require.True(t, ref.IsCountingVotes(101))
	require.True(t, ref.IsCountingVotes(199))
	require.False(t, ref.IsCountingVotes(200))
	require.False(t, ref.IsCountingVotes(201))

	// Verify ReferendumShouldAcceptVotes
	require.False(t, ref.ShouldAcceptVotes(89))
	require.False(t, ref.ShouldAcceptVotes(90))
	require.True(t, ref.ShouldAcceptVotes(91))
	require.True(t, ref.ShouldAcceptVotes(99))
	require.True(t, ref.ShouldAcceptVotes(100))
	require.True(t, ref.ShouldAcceptVotes(101))
	require.True(t, ref.ShouldAcceptVotes(199))
	require.True(t, ref.ShouldAcceptVotes(200))
	require.False(t, ref.ShouldAcceptVotes(201))

	// Verify ReferendumShouldCountVotes
	require.False(t, ref.ShouldCountVotes(89))
	require.False(t, ref.ShouldCountVotes(90))
	require.False(t, ref.ShouldCountVotes(91))
	require.False(t, ref.ShouldCountVotes(99))
	require.False(t, ref.ShouldCountVotes(100))
	require.True(t, ref.ShouldCountVotes(101))
	require.True(t, ref.ShouldCountVotes(199))
	require.True(t, ref.ShouldCountVotes(200))
	require.False(t, ref.ShouldCountVotes(201))
}

func TestReferendumStates(t *testing.T) {
	env := test.NewReferendumTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	require.Empty(t, env.ReferendumManager().Referendums())
	referendumID := env.RegisterDefaultReferendum(5, 1, 2)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(6), ref.StartHoldingMilestoneIndex())
	require.Equal(t, milestone.Index(8), ref.EndMilestoneIndex())

	// No referendum should be running right now
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	env.AssertReferendumsCount(0, 0)

	env.IssueMilestone() // 5

	// Referendum should be accepting votes, but not counting
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	env.AssertReferendumsCount(1, 0)

	env.IssueMilestone() // 6

	// Referendum should be accepting and counting votes
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	env.AssertReferendumsCount(1, 1)

	env.IssueMilestone() // 7

	// Referendum should be ended
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	env.AssertReferendumsCount(1, 1)

	env.IssueMilestone() // 8

	// Referendum should be ended
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	env.AssertReferendumsCount(0, 0)
}

func TestSingleReferendumVote(t *testing.T) {
	env := test.NewReferendumTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 3)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartHoldingMilestoneIndex())
	require.Equal(t, milestone.Index(10), ref.EndMilestoneIndex())

	// Referendum should not be accepting votes yet
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))

	// Issue a vote and milestone
	env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 5

	// Votes should not have been counted so far because it was not accepting votes yet
	status, err := env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	// Referendum should be accepting votes now
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))

	// Vote again
	castVote := env.IssueDefaultVoteAndMilestone(referendumID, env.Wallet1) // 6

	// Referendum should be accepting votes, but the vote should not be weighted yet, just added to the current status
	env.AssertReferendumsCount(1, 0)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 7

	// Referendum should be accepting and counting votes, but the vote we did before should not be weighted yet
	env.AssertReferendumsCount(1, 1)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 8

	// Referendum should be accepting and counting votes, the vote should now be weighted
	env.AssertReferendumsCount(1, 1)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 9

	// Referendum should be accepting and counting votes, the vote should now be weighted double
	env.AssertReferendumsCount(1, 1)

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Equal(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Equal(t, uint64(2_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 10

	// Referendum should be ended
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
	env := test.NewReferendumTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartHoldingMilestoneIndex())
	require.Equal(t, milestone.Index(12), ref.EndMilestoneIndex())

	// Referendum should not be accepting votes yet
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))

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

	// Issue final milestone that ends the referendum
	env.IssueMilestone() // 12

	// There should be no active votes left
	env.AssertReferendumVoteStatus(referendumID, 0, 3)
	env.AssertDefaultBallotAnswerStatus(referendumID, 1_000_000, 3_000_000)

	// Verify the vote history after the referendum ended
	env.AssertTrackedVote(referendumID, castVote1, 6, 7, 1_000_000)
	env.AssertTrackedVote(referendumID, castVote2, 8, 9, 1_000_000)
	env.AssertTrackedVote(referendumID, castVote3, 11, 12, 1_000_000)
}

func TestReferendumVoteAddVoteBalanceBySweeping(t *testing.T) {
	env := test.NewReferendumTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartHoldingMilestoneIndex())
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
	env := test.NewReferendumTestEnv(t, 5_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterDefaultReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.StartMilestoneIndex())
	require.Equal(t, milestone.Index(7), ref.StartHoldingMilestoneIndex())
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
