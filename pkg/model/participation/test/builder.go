package test

import (
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/serializer/v2"

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
	msgBuilder := env.te.NewMessageBuilder(ParticipationTag).
		LatestMilestoneAsParents()

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

func (b *ParticipationHelper) Build() *testsuite.Message {
	votes, err := b.participationsBuilder.Build()
	require.NoError(b.env.t, err)

	participationsData, err := votes.Serialize(serializer.DeSeriModePerformValidation, nil)
	require.NoError(b.env.t, err)

	msg := b.msgBuilder.
		FromWallet(b.wallet).
		ToWallet(b.wallet).
		TagData(participationsData).
		Build()

	return msg
}

func (b *ParticipationHelper) Send() *SentParticipations {
	return &SentParticipations{
		builder: b,
		message: b.Build().Store().BookOnWallets(),
	}
}

func (c *SentParticipations) Message() *testsuite.Message {
	return c.message
}
