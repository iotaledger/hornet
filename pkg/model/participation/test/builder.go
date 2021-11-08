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

type ParticipationHelper struct {
	env                   *ParticipationTestEnv
	wallet                *utils.HDWallet
	msgBuilder            *testsuite.MessageBuilder
	participationsBuilder *participation.ParticipationsBuilder
}

type SentParticipations struct {
	builder *ParticipationHelper
	message *testsuite.Message
}

func (env *ParticipationTestEnv) NewParticipationHelper(wallet *utils.HDWallet) *ParticipationHelper {
	msgBuilder := env.te.NewMessageBuilder(participationIndexation).
		LatestMilestonesAsParents()

	return &ParticipationHelper{
		env:                   env,
		wallet:                wallet,
		msgBuilder:            msgBuilder,
		participationsBuilder: participation.NewParticipationsBuilder(),
	}
}

func (b *ParticipationHelper) WholeWalletBalance() *ParticipationHelper {
	b.msgBuilder.Amount(b.wallet.Balance())
	return b
}

func (b *ParticipationHelper) Amount(amount uint64) *ParticipationHelper {
	b.msgBuilder.Amount(amount)
	return b
}

func (b *ParticipationHelper) Parents(parents hornet.MessageIDs) *ParticipationHelper {
	require.NotEmpty(b.env.t, parents)
	b.msgBuilder.Parents(parents)
	return b
}

func (b *ParticipationHelper) UsingOutput(output *utxo.Output) *ParticipationHelper {
	require.NotNil(b.env.t, output)
	b.msgBuilder.UsingOutput(output)
	return b
}

func (b *ParticipationHelper) AddParticipations(participations []*participation.Participation) *ParticipationHelper {
	require.NotEmpty(b.env.t, participations)
	for _, p := range participations {
		b.AddParticipation(p)
	}
	return b
}

func (b *ParticipationHelper) AddDefaultBallotVote(eventID participation.EventID) *ParticipationHelper {
	b.participationsBuilder.AddParticipation(&participation.Participation{
		EventID: eventID,
		Answers: []byte{defaultBallotAnswerValue},
	})
	return b
}

func (b *ParticipationHelper) AddParticipation(participation *participation.Participation) *ParticipationHelper {
	require.NotNil(b.env.t, participation)
	b.participationsBuilder.AddParticipation(participation)
	return b
}

func (b *ParticipationHelper) Send() *SentParticipations {
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

	return &SentParticipations{
		builder: b,
		message: msg,
	}
}

func (c *SentParticipations) Message() *testsuite.Message {
	return c.message
}
