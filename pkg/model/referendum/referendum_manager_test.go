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

	referendumBuilder.AddQuestion(question)

	ref, err := referendumBuilder.Build()
	require.NoError(t, err)

	// Verify ReferendumIsAcceptingVotes
	require.False(t, referendum.ReferendumIsAcceptingVotes(ref, 89))
	require.True(t, referendum.ReferendumIsAcceptingVotes(ref, 90))
	require.True(t, referendum.ReferendumIsAcceptingVotes(ref, 91))
	require.True(t, referendum.ReferendumIsAcceptingVotes(ref, 99))
	require.True(t, referendum.ReferendumIsAcceptingVotes(ref, 100))
	require.True(t, referendum.ReferendumIsAcceptingVotes(ref, 101))
	require.True(t, referendum.ReferendumIsAcceptingVotes(ref, 199))
	require.False(t, referendum.ReferendumIsAcceptingVotes(ref, 200))
	require.False(t, referendum.ReferendumIsAcceptingVotes(ref, 201))

	// Verify ReferendumIsCountingVotes
	require.False(t, referendum.ReferendumIsCountingVotes(ref, 89))
	require.False(t, referendum.ReferendumIsCountingVotes(ref, 90))
	require.False(t, referendum.ReferendumIsCountingVotes(ref, 91))
	require.False(t, referendum.ReferendumIsCountingVotes(ref, 99))
	require.True(t, referendum.ReferendumIsCountingVotes(ref, 100))
	require.True(t, referendum.ReferendumIsCountingVotes(ref, 101))
	require.True(t, referendum.ReferendumIsCountingVotes(ref, 199))
	require.False(t, referendum.ReferendumIsCountingVotes(ref, 200))
	require.False(t, referendum.ReferendumIsCountingVotes(ref, 201))

	// Verify ReferendumShouldAcceptVotes
	require.False(t, referendum.ReferendumShouldAcceptVotes(ref, 89))
	require.False(t, referendum.ReferendumShouldAcceptVotes(ref, 90))
	require.True(t, referendum.ReferendumShouldAcceptVotes(ref, 91))
	require.True(t, referendum.ReferendumShouldAcceptVotes(ref, 99))
	require.True(t, referendum.ReferendumShouldAcceptVotes(ref, 100))
	require.True(t, referendum.ReferendumShouldAcceptVotes(ref, 101))
	require.True(t, referendum.ReferendumShouldAcceptVotes(ref, 199))
	require.True(t, referendum.ReferendumShouldAcceptVotes(ref, 200))
	require.False(t, referendum.ReferendumShouldAcceptVotes(ref, 201))

	// Verify ReferendumShouldCountVotes
	require.False(t, referendum.ReferendumShouldCountVotes(ref, 89))
	require.False(t, referendum.ReferendumShouldCountVotes(ref, 90))
	require.False(t, referendum.ReferendumShouldCountVotes(ref, 91))
	require.False(t, referendum.ReferendumShouldCountVotes(ref, 99))
	require.False(t, referendum.ReferendumShouldCountVotes(ref, 100))
	require.True(t, referendum.ReferendumShouldCountVotes(ref, 101))
	require.True(t, referendum.ReferendumShouldCountVotes(ref, 199))
	require.True(t, referendum.ReferendumShouldCountVotes(ref, 200))
	require.False(t, referendum.ReferendumShouldCountVotes(ref, 201))
}

func TestReferendumStates(t *testing.T) {
	env := test.NewReferendumTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	require.Empty(t, env.ReferendumManager().Referendums())
	referendumID := env.RegisterSampleReferendum(5, 1, 2)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.MilestoneStart)
	require.Equal(t, milestone.Index(6), ref.MilestoneStartHolding)
	require.Equal(t, milestone.Index(8), ref.MilestoneEnd)

	// No referendum should be running right now
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsCountingVotes()))

	env.IssueMilestone() // 5

	// Referendum should be accepting votes, but not counting
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsCountingVotes()))

	env.IssueMilestone() // 6

	// Referendum should be accepting and counting votes
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsCountingVotes()))

	env.IssueMilestone() // 7

	// Referendum should be ended
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsCountingVotes()))

	env.IssueMilestone() // 8

	// Referendum should be ended
	require.Equal(t, 1, len(env.ReferendumManager().Referendums()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsCountingVotes()))
}

func TestSingleReferendumVote(t *testing.T) {
	env := test.NewReferendumTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterSampleReferendum(5, 2, 3)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.MilestoneStart)
	require.Equal(t, milestone.Index(7), ref.MilestoneStartHolding)
	require.Equal(t, milestone.Index(10), ref.MilestoneEnd)

	// Referendum should not be accepting votes yet
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))

	// Issue a vote and milestone
	env.IssueSampleVoteAndMilestone(referendumID, env.Wallet1) // 5

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
	votingMessage := env.IssueSampleVoteAndMilestone(referendumID, env.Wallet1) // 6

	// Referendum should be accepting votes, but the vote should not be weighted yet, just added to the current status
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsCountingVotes()))

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 7

	// Referendum should be accepting and counting votes, but the vote we did before should not be weighted yet
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsCountingVotes()))

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 8

	// Referendum should be accepting and counting votes, the vote should now be weighted
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsCountingVotes()))

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 9

	// Referendum should be accepting and counting votes, the vote should now be weighted double
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.ReferendumManager().ReferendumsCountingVotes()))

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Equal(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Equal(t, uint64(2_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 10

	// Referendum should be ended
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsCountingVotes()))

	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Equal(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Equal(t, uint64(3_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 11

	trackedVote, err := env.ReferendumManager().VoteForOutputID(referendumID, votingMessage.GeneratedUTXO().OutputID())
	require.Equal(t, votingMessage.StoredMessageID(), trackedVote.MessageID)
	require.Equal(t, milestone.Index(6), trackedVote.StartIndex)
	require.Equal(t, milestone.Index(0), trackedVote.EndIndex) // was never spent, so the vote is still valid, although the referendum ended

	messageFromReferendum, err := env.ReferendumManager().MessageForMessageID(trackedVote.MessageID)
	require.NoError(t, err)
	require.NotNil(t, messageFromReferendum)
	require.Equal(t, messageFromReferendum.Message(), votingMessage.IotaMessage())
}

func TestReferendumVoteCancel(t *testing.T) {
	env := test.NewReferendumTestEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.RegisterSampleReferendum(5, 2, 5)

	ref := env.ReferendumManager().Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.MilestoneStart)
	require.Equal(t, milestone.Index(7), ref.MilestoneStartHolding)
	require.Equal(t, milestone.Index(12), ref.MilestoneEnd)

	// Referendum should not be accepting votes yet
	require.Equal(t, 0, len(env.ReferendumManager().ReferendumsAcceptingVotes()))

	env.IssueMilestone() // 5

	// Issue a vote and milestone
	vote1Msg := env.IssueSampleVoteAndMilestone(referendumID, env.Wallet1) // 6

	// Verify vote
	status, err := env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	// Cancel vote
	cancelVote1Msg := env.CancelVote(env.Wallet1)
	env.IssueMilestone(cancelVote1Msg.StoredMessageID(), env.LastMilestoneMessageID()) // 7

	// Verify vote
	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	// Vote again
	vote2Msg := env.IssueSampleVoteAndMilestone(referendumID, env.Wallet1) // 8

	// Verify vote
	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	// Cancel vote
	cancelVote2Msg := env.CancelVote(env.Wallet1)
	env.IssueMilestone(cancelVote2Msg.StoredMessageID(), env.LastMilestoneMessageID()) // 9

	// Verify vote
	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 10

	// Verify vote
	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	// Vote again
	vote3Msg := env.IssueSampleVoteAndMilestone(referendumID, env.Wallet1) // 11

	// Verify vote
	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(2_000_000), status.Questions[0].Answers[1].Accumulated)

	require.Equal(t, 1, len(env.ActiveVotesForReferendum(referendumID)))
	require.Equal(t, 2, len(env.PastVotesForReferendum(referendumID)))

	// Verify the last issued vote is still active, i.e. EndIndex == 0
	lastIssuedVote, err := env.ReferendumManager().VoteForOutputID(referendumID, vote3Msg.GeneratedUTXO().OutputID())
	require.NoError(t, err)
	require.Equal(t, vote3Msg.GeneratedUTXO().OutputID(), lastIssuedVote.OutputID)
	require.Equal(t, vote3Msg.StoredMessageID(), lastIssuedVote.MessageID)
	require.Equal(t, uint64(1_000_000), lastIssuedVote.Amount)
	require.Equal(t, milestone.Index(11), lastIssuedVote.StartIndex)
	require.Equal(t, milestone.Index(0), lastIssuedVote.EndIndex)

	// Issue final milestone that ends the referendum
	env.IssueMilestone() // 12

	// There should be no active votes left
	require.Equal(t, 0, len(env.ActiveVotesForReferendum(referendumID)))
	require.Equal(t, 3, len(env.PastVotesForReferendum(referendumID)))

	// Verify vote
	status, err = env.ReferendumManager().ReferendumStatus(referendumID)
	require.NoError(t, err)
	env.PrintJSON(status)
	require.Equal(t, env.ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(3_000_000), status.Questions[0].Answers[1].Accumulated)

	// Verify the vote history after the referendum ended
	trackedVote1, err := env.ReferendumManager().VoteForOutputID(referendumID, vote1Msg.GeneratedUTXO().OutputID())
	require.NoError(t, err)
	require.Equal(t, vote1Msg.GeneratedUTXO().OutputID(), trackedVote1.OutputID)
	require.Equal(t, vote1Msg.StoredMessageID(), trackedVote1.MessageID)
	require.Equal(t, uint64(1_000_000), trackedVote1.Amount)
	require.Equal(t, milestone.Index(6), trackedVote1.StartIndex)
	require.Equal(t, milestone.Index(7), trackedVote1.EndIndex)

	trackedVote2, err := env.ReferendumManager().VoteForOutputID(referendumID, vote2Msg.GeneratedUTXO().OutputID())
	require.NoError(t, err)
	require.Equal(t, vote2Msg.GeneratedUTXO().OutputID(), trackedVote2.OutputID)
	require.Equal(t, vote2Msg.StoredMessageID(), trackedVote2.MessageID)
	require.Equal(t, uint64(1_000_000), trackedVote2.Amount)
	require.Equal(t, milestone.Index(8), trackedVote2.StartIndex)
	require.Equal(t, milestone.Index(9), trackedVote2.EndIndex)

	trackedVote3, err := env.ReferendumManager().VoteForOutputID(referendumID, vote3Msg.GeneratedUTXO().OutputID())
	require.NoError(t, err)
	require.Equal(t, vote3Msg.GeneratedUTXO().OutputID(), trackedVote3.OutputID)
	require.Equal(t, vote3Msg.StoredMessageID(), trackedVote3.MessageID)
	require.Equal(t, uint64(1_000_000), trackedVote3.Amount)
	require.Equal(t, milestone.Index(11), trackedVote3.StartIndex)
	require.Equal(t, milestone.Index(12), trackedVote3.EndIndex)
}
