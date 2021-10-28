package test

import (
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/referendum"
	"github.com/iotaledger/hive.go/serializer"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
)

type VoteBuilder struct {
	env          *ReferendumTestEnv
	msgBuilder   *testsuite.MessageBuilder
	votesBuilder *referendum.VotesBuilder
}

type CastVote struct {
	builder *VoteBuilder
	message *testsuite.Message
}

func (env *ReferendumTestEnv) NewVoteBuilder(wallet *utils.HDWallet) *VoteBuilder {
	msgBuilder := env.te.NewMessageBuilder(voteIndexation).
		LatestMilestonesAsParents().
		FromWallet(wallet).
		ToWallet(wallet)

	return &VoteBuilder{
		env:          env,
		msgBuilder:   msgBuilder,
		votesBuilder: referendum.NewVotesBuilder(),
	}
}

func (b *VoteBuilder) Amount(amount uint64) *VoteBuilder {
	b.msgBuilder.Amount(amount)
	return b
}

func (b *VoteBuilder) Parents(parents hornet.MessageIDs) *VoteBuilder {
	require.NotEmpty(b.env.t, parents)
	b.msgBuilder.Parents(parents)
	return b
}

func (b *VoteBuilder) UsingOutput(output *utxo.Output) *VoteBuilder {
	require.NotNil(b.env.t, output)
	b.msgBuilder.UsingOutput(output)
	return b
}

func (b *VoteBuilder) AddVotes(votes []*referendum.Vote) *VoteBuilder {
	require.NotEmpty(b.env.t, votes)
	for _, vote := range votes {
		b.AddVote(vote)
	}
	return b
}

func (b *VoteBuilder) AddVote(vote *referendum.Vote) *VoteBuilder {
	require.NotNil(b.env.t, vote)
	b.votesBuilder.AddVote(vote)
	return b
}

func (b *VoteBuilder) Cast() *CastVote {
	votes, err := b.votesBuilder.Build()
	require.NoError(b.env.t, err)

	votesData, err := votes.Serialize(serializer.DeSeriModePerformValidation)
	require.NoError(b.env.t, err)

	msg := b.msgBuilder.
		IndexationData(votesData).
		Build().
		Store().
		BookOnWallets()

	return &CastVote{
		builder: b,
		message: msg,
	}
}

func (c *CastVote) Message() *testsuite.Message {
	return c.message
}
