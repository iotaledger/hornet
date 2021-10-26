package referendum_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/referendum"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	genesisSeed, _ = hex.DecodeString("2f54b071657e6644629a40518ba6554de4eee89f0757713005ad26137d80968d05e1ca1bca555d8b4b85a3f4fcf11a6a48d3d628d1ace40f48009704472fc8f9")
	seed1, _       = hex.DecodeString("96d9ff7a79e4b0a5f3e5848ae7867064402da92a62eabb4ebbe463f12d1f3b1aace1775488f51cb1e3a80732a03ef60b111d6833ab605aa9f8faebeb33bbe3d9")
	seed2, _       = hex.DecodeString("b15209ddc93cbdb600137ea6a8f88cdd7c5d480d5815c9352a0fb5c4e4b86f7151dcb44c2ba635657a2df5a8fd48cb9bab674a9eceea527dbbb254ef8c9f9cd7")
	seed3, _       = hex.DecodeString("d5353ceeed380ab89a0f6abe4630c2091acc82617c0edd4ff10bd60bba89e2ed30805ef095b989c2bf208a474f8748d11d954aade374380422d4d812b6f1da90")
	seed4, _       = hex.DecodeString("bd6fe09d8a309ca309c5db7b63513240490109cd0ac6b123551e9da0d5c8916c4a5a4f817e4b4e9df89885ce1af0986da9f1e56b65153c2af1e87ab3b11dabb4")

	MinPoWScore   = 100.0
	BelowMaxDepth = 15

	voteIndexation = "TESTVOTE"
)

type testEnv struct {
	t  *testing.T
	te *testsuite.TestEnvironment

	genesisWallet *utils.HDWallet
	wallet1       *utils.HDWallet
	wallet2       *utils.HDWallet
	wallet3       *utils.HDWallet
	wallet4       *utils.HDWallet

	referendumStore kvstore.KVStore
	rm              *referendum.ReferendumManager
}

func newEnv(t *testing.T, wallet1Balance uint64, wallet2Balance uint64, wallet3Balance uint64, wallet4Balance uint64, assertSteps bool) *testEnv {

	genesisWallet := utils.NewHDWallet("Genesis", genesisSeed, 0)
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)
	seed4Wallet := utils.NewHDWallet("Seed4", seed4, 0)

	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, BelowMaxDepth, MinPoWScore, false)

	//Add token supply to our local HDWallet
	genesisWallet.BookOutput(te.GenesisOutput)
	if assertSteps {
		te.AssertWalletBalance(genesisWallet, 2_779_530_283_277_761)
	}

	// Fund wallet1
	messageA := te.NewMessageBuilder("A").
		Parents(hornet.MessageIDs{te.Milestones[0].Milestone().MessageID, te.Milestones[1].Milestone().MessageID}).
		FromWallet(genesisWallet).
		ToWallet(seed1Wallet).
		Amount(wallet1Balance).
		Build().
		Store().
		BookOnWallets()

	// Fund wallet2
	messageB := te.NewMessageBuilder("B").
		Parents(hornet.MessageIDs{messageA.StoredMessageID(), te.Milestones[1].Milestone().MessageID}).
		FromWallet(genesisWallet).
		ToWallet(seed2Wallet).
		Amount(wallet2Balance).
		Build().
		Store().
		BookOnWallets()

	// Fund wallet3
	messageC := te.NewMessageBuilder("C").
		Parents(hornet.MessageIDs{messageB.StoredMessageID(), te.Milestones[1].Milestone().MessageID}).
		FromWallet(genesisWallet).
		ToWallet(seed3Wallet).
		Amount(wallet3Balance).
		Build().
		Store().
		BookOnWallets()

	// Fund wallet4
	messageD := te.NewMessageBuilder("D").
		Parents(hornet.MessageIDs{messageC.StoredMessageID(), te.Milestones[1].Milestone().MessageID}).
		FromWallet(genesisWallet).
		ToWallet(seed4Wallet).
		Amount(wallet4Balance).
		Build().
		Store().
		BookOnWallets()

	// Confirming milestone at message D
	_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageD.StoredMessageID()}, false)
	if assertSteps {

		require.Equal(t, 4+1, confStats.MessagesReferenced) // 4 + milestone itself
		require.Equal(t, 4, confStats.MessagesIncludedWithTransactions)
		require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
		require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

		//Verify balances
		te.AssertWalletBalance(genesisWallet, 2_779_530_283_277_761-wallet1Balance-wallet2Balance-wallet3Balance-wallet4Balance)
		te.AssertWalletBalance(seed1Wallet, wallet1Balance)
		te.AssertWalletBalance(seed2Wallet, wallet2Balance)
		te.AssertWalletBalance(seed3Wallet, wallet3Balance)
		te.AssertWalletBalance(seed4Wallet, wallet4Balance)
	}

	referendumStore := mapdb.NewMapDB()

	rm, err := referendum.NewManager(
		te.Storage(),
		te.SyncManager(),
		referendumStore,
		referendum.WithIndexationMessage(voteIndexation),
	)
	require.NoError(t, err)

	// Connect the callbacks from the testsuite to the ReferendumManager
	te.ConfigureUTXOCallbacks(
		func(index milestone.Index, output *utxo.Output) {
			require.NoError(t, rm.ApplyNewUTXO(index, output))
		},
		func(index milestone.Index, spent *utxo.Spent) {
			require.NoError(t, rm.ApplySpentUTXO(index, spent))
		},
		func(index milestone.Index) {
			require.NoError(t, rm.ApplyNewConfirmedMilestoneIndex(index))
		},
	)

	return &testEnv{
		t:               t,
		te:              te,
		genesisWallet:   genesisWallet,
		wallet1:         seed1Wallet,
		wallet2:         seed2Wallet,
		wallet3:         seed3Wallet,
		wallet4:         seed4Wallet,
		referendumStore: referendumStore,
		rm:              rm,
	}
}

func (env *testEnv) Cleanup() {
	env.rm.CloseDatabase()
	env.te.CleanupTestEnvironment(true)
}

func (env *testEnv) IssueVote(wallet *utils.HDWallet, amount uint64, votes *referendum.Votes) *testsuite.Message {

	require.LessOrEqualf(env.t, amount, wallet.Balance(), "trying to vote with more than available in the wallet")

	votesData, err := votes.Serialize(iotago.DeSeriModePerformValidation)
	require.NoError(env.t, err)

	return env.te.NewMessageBuilder(voteIndexation).
		Parents(hornet.MessageIDs{env.te.Milestones[len(env.te.Milestones)-1].Milestone().MessageID, env.te.Milestones[len(env.te.Milestones)-2].Milestone().MessageID}).
		FromWallet(wallet).
		ToWallet(wallet).
		Amount(amount).
		IndexationData(votesData).
		Build().
		Store().
		BookOnWallets()
}

// TestTestEnv verifies that our testEnv is sane. This allows us to skip the assertions on the other tests to speed them up
func TestTestEnv(t *testing.T) {

	randomBalance := func() uint64 {
		return uint64(rand.Intn(256)) * 1_000_000
	}

	env := newEnv(t, randomBalance(), randomBalance(), randomBalance(), randomBalance(), true)
	defer env.Cleanup()
	require.NotNil(t, env)
}

func (env *testEnv) registerSampleReferendum(startMilestoneIndex milestone.Index, startPhaseDuration milestone.Index, holdingDuration milestone.Index) referendum.ReferendumID {

	referendumStartIndex := startMilestoneIndex
	referendumStartHoldingIndex := referendumStartIndex + startPhaseDuration
	referendumEndIndex := referendumStartHoldingIndex + holdingDuration

	referendumBuilder := referendum.NewReferendumBuilder("All 4 HORNET", referendumStartIndex, referendumStartHoldingIndex, referendumEndIndex, "The biggest governance decision in the history of IOTA")

	questionBuilder := referendum.NewQuestionBuilder("Give all the funds to the HORNET developers?", "This would fund the development of HORNET indefinitely")
	questionBuilder.AddAnswer(&referendum.Answer{
		Index:          1,
		Text:           "YES",
		AdditionalInfo: "Go team!",
	})
	questionBuilder.AddAnswer(&referendum.Answer{
		Index:          2,
		Text:           "Doh! Of course!",
		AdditionalInfo: "There is no other option",
	})

	question, err := questionBuilder.Build()
	require.NoError(env.t, err)

	referendumBuilder.AddQuestion(question)

	ref, err := referendumBuilder.Build()
	require.NoError(env.t, err)

	referendumID, err := env.rm.StoreReferendum(ref)
	require.NoError(env.t, err)

	// Check the stored referendum is still there
	require.NotNil(env.t, env.rm.Referendum(referendumID))

	printJSON(env.t, ref)

	return referendumID
}

func (env *testEnv) issueSampleVoteAndMilestone(referendumID referendum.ReferendumID, wallet *utils.HDWallet) {

	votesBuilder := referendum.NewVotesBuilder()
	votesBuilder.AddVote(&referendum.Vote{
		ReferendumID: referendumID,
		Answers:      []byte{byte(1)},
	})

	votes, err := votesBuilder.Build()
	require.NoError(env.t, err)

	message := env.IssueVote(wallet, wallet.Balance(), votes)

	_, confStats := env.te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{message.StoredMessageID()}, false)
	require.Equal(env.t, 1+1, confStats.MessagesReferenced) // 1 + milestone itself
}

func (env *testEnv) IssueMilestone() {
	env.te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{env.te.LastMilestoneMessageID}, false)
}

func printJSON(t *testing.T, i interface{}) {
	j, err := json.MarshalIndent(i, "", "  ")
	require.NoError(t, err)
	fmt.Println(string(j))
}

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
	env := newEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.te.SyncManager().ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	require.Empty(t, env.rm.Referendums())
	referendumID := env.registerSampleReferendum(5, 1, 2)

	ref := env.rm.Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.MilestoneStart)
	require.Equal(t, milestone.Index(6), ref.MilestoneStartHolding)
	require.Equal(t, milestone.Index(8), ref.MilestoneEnd)

	// No referendum should be running right now
	require.Equal(t, 1, len(env.rm.Referendums()))
	require.Equal(t, 0, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.rm.ReferendumsCountingVotes()))

	env.IssueMilestone() // 5

	// Referendum should be accepting votes, but not counting
	require.Equal(t, 1, len(env.rm.Referendums()))
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.rm.ReferendumsCountingVotes()))

	env.IssueMilestone() // 6

	// Referendum should be accepting and counting votes
	require.Equal(t, 1, len(env.rm.Referendums()))
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.rm.ReferendumsCountingVotes()))

	env.IssueMilestone() // 7

	// Referendum should be ended
	require.Equal(t, 1, len(env.rm.Referendums()))
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.rm.ReferendumsCountingVotes()))

	env.IssueMilestone() // 8

	// Referendum should be ended
	require.Equal(t, 1, len(env.rm.Referendums()))
	require.Equal(t, 0, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.rm.ReferendumsCountingVotes()))
}

func TestSingleReferendumVote(t *testing.T) {
	env := newEnv(t, 1_000_000, 150_000_000, 200_000_000, 300_000_000, false)
	defer env.Cleanup()

	confirmedMilestoneIndex := env.te.SyncManager().ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	referendumID := env.registerSampleReferendum(5, 2, 3)

	ref := env.rm.Referendum(referendumID)
	require.NotNil(t, ref)

	// Verify the configured referendum indexes
	require.Equal(t, milestone.Index(5), ref.MilestoneStart)
	require.Equal(t, milestone.Index(7), ref.MilestoneStartHolding)
	require.Equal(t, milestone.Index(10), ref.MilestoneEnd)

	// Referendum should not be accepting votes yet
	require.Equal(t, 0, len(env.rm.ReferendumsAcceptingVotes()))

	// Issue a vote and milestone
	env.issueSampleVoteAndMilestone(referendumID, env.wallet1) // 5

	// Votes should not have been counted so far because it was not accepting votes yet
	status, err := env.rm.ReferendumStatus(referendumID)
	require.NoError(t, err)
	printJSON(t, status)
	require.Equal(t, env.te.SyncManager().ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	// Referendum should be accepting votes now
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))

	// Vote again
	env.issueSampleVoteAndMilestone(referendumID, env.wallet1) // 6

	// Referendum should be accepting votes, but the vote should not be weighted yet, just added to the current status
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.rm.ReferendumsCountingVotes()))

	status, err = env.rm.ReferendumStatus(referendumID)
	require.NoError(t, err)
	printJSON(t, status)
	require.Equal(t, env.te.SyncManager().ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 7

	// Referendum should be accepting and counting votes, but the vote we did before should not be weighted yet
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.rm.ReferendumsCountingVotes()))

	status, err = env.rm.ReferendumStatus(referendumID)
	require.NoError(t, err)
	printJSON(t, status)
	require.Equal(t, env.te.SyncManager().ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(0), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 8

	// Referendum should be accepting and counting votes, the vote should now be weighted
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.rm.ReferendumsCountingVotes()))

	status, err = env.rm.ReferendumStatus(referendumID)
	require.NoError(t, err)
	printJSON(t, status)
	require.Equal(t, env.te.SyncManager().ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Exactly(t, uint64(1_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 9

	// Referendum should be accepting and counting votes, the vote should now be weighted double
	require.Equal(t, 1, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 1, len(env.rm.ReferendumsCountingVotes()))

	status, err = env.rm.ReferendumStatus(referendumID)
	require.NoError(t, err)
	printJSON(t, status)
	require.Equal(t, env.te.SyncManager().ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Equal(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Equal(t, uint64(2_000_000), status.Questions[0].Answers[1].Accumulated)

	env.IssueMilestone() // 10

	// Referendum should be ended
	require.Equal(t, 0, len(env.rm.ReferendumsAcceptingVotes()))
	require.Equal(t, 0, len(env.rm.ReferendumsCountingVotes()))

	status, err = env.rm.ReferendumStatus(referendumID)
	require.NoError(t, err)
	printJSON(t, status)
	require.Equal(t, env.te.SyncManager().ConfirmedMilestoneIndex(), status.MilestoneIndex)
	require.Equal(t, uint64(1_000_000), status.Questions[0].Answers[1].Current)
	require.Equal(t, uint64(3_000_000), status.Questions[0].Answers[1].Accumulated)
}
