package test

import (
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/serializer"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
)

type ParticipationsBuilder struct {
	env                   *ParticipationTestEnv
	wallet                *utils.HDWallet
	msgBuilder            *testsuite.MessageBuilder
	participationsBuilder *participation.ParticipationsBuilder
}

type SentParticipation struct {
	builder *ParticipationsBuilder
	message *testsuite.Message
}

func (env *ParticipationTestEnv) NewParticipationsBuilder(wallet *utils.HDWallet) *ParticipationsBuilder {
	msgBuilder := env.te.NewMessageBuilder(voteIndexation).
		LatestMilestonesAsParents()

	return &ParticipationsBuilder{
		env:                   env,
		wallet:                wallet,
		msgBuilder:            msgBuilder,
		participationsBuilder: participation.NewParticipationsBuilder(),
	}
}

func (b *ParticipationsBuilder) WholeWalletBalance() *ParticipationsBuilder {
	b.msgBuilder.Amount(b.wallet.Balance())
	return b
}

func (b *ParticipationsBuilder) Amount(amount uint64) *ParticipationsBuilder {
	b.msgBuilder.Amount(amount)
	return b
}

func (b *ParticipationsBuilder) Parents(parents hornet.MessageIDs) *ParticipationsBuilder {
	require.NotEmpty(b.env.t, parents)
	b.msgBuilder.Parents(parents)
	return b
}

func (b *ParticipationsBuilder) UsingOutput(output *utxo.Output) *ParticipationsBuilder {
	require.NotNil(b.env.t, output)
	b.msgBuilder.UsingOutput(output)
	return b
}

func (b *ParticipationsBuilder) AddParticipations(participations []*participation.Participation) *ParticipationsBuilder {
	require.NotEmpty(b.env.t, participations)
	for _, p := range participations {
		b.AddParticipation(p)
	}
	return b
}

func (b *ParticipationsBuilder) AddDefaultBallotVote(eventID participation.ParticipationEventID) *ParticipationsBuilder {
	b.participationsBuilder.AddVote(&participation.Participation{
		ParticipationEventID: eventID,
		Answers:              []byte{byte(1)},
	})
	return b
}

func (b *ParticipationsBuilder) AddParticipation(participation *participation.Participation) *ParticipationsBuilder {
	require.NotNil(b.env.t, participation)
	b.participationsBuilder.AddVote(participation)
	return b
}

func (b *ParticipationsBuilder) Participate() *SentParticipation {
	votes, err := b.participationsBuilder.Build()
	require.NoError(b.env.t, err)

	participationsData, err := votes.Serialize(serializer.DeSeriModePerformValidation)
	require.NoError(b.env.t, err)

	msg := b.msgBuilder.
		FromWallet(b.wallet).
		ToWallet(b.wallet).
		IndexationData(participationsData).
		Build().
		Store().
		BookOnWallets()

	return &SentParticipation{
		builder: b,
		message: msg,
	}
}

func (c *SentParticipation) Message() *testsuite.Message {
	return c.message
}
