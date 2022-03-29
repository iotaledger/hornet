package participation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/gohornet/hornet/pkg/model/participation/test"
	iotago "github.com/iotaledger/iota.go/v3"
)

/*
A: (5 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 14 15 (event end)
B: (3 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 (spent) 14 15 (event end)
C: (3+1 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 (spent) 14 (staked) 15 (event end)
D: (3+1 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 (spent) (staked) 14 (spent) 15 (event end)
E: (5 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 14 15 (event end)
F: (3 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 (spent) 14 15 (event end)
G: (3+2 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 (spent) (staked) 14 15 (event end)
H: (3+1 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 (spent) (staked) 14 (spent) 15 (event end)
I: (3 Milestones) = 6 7 8 9 10 (event start) 11 12 (staked) 13 14 15 (event end)
J: (1 Milestones) = 6 7 8 9 10 (event start) 11 12 (staked) 13 (spent) 14 15 (event end)
K: (1+2 Milestones) = 6 7 8 9 10 (event start) 11 12 (staked) 13 (spent) (staked) 14 15 (event end)
*/

type stakingTestEnv struct {
	env     *test.ParticipationTestEnv
	eventID participation.EventID
}

func stakingEnv(t *testing.T) *stakingTestEnv {
	env := test.NewParticipationTestEnv(t, 1_000_000, 1_000_000, 1_000_000, 1_000_000, false)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	eventBuilder := participation.NewEventBuilder("AlbinoPugCoin", 6, 10, 15, "The first DogCoin on the Tangle")
	eventBuilder.Payload(&participation.Staking{
		Text:           "The rarest DogCoin on earth",
		Symbol:         "APUG",
		Numerator:      1,
		Denominator:    1,
		AdditionalInfo: "Have you seen an albino Pug?",
	})

	event, err := eventBuilder.Build()
	require.NoError(t, err)

	eventID, err := env.ParticipationManager().StoreEvent(event)
	require.NoError(t, err)

	// Verify the configured indexes
	require.Equal(t, milestone.Index(6), event.CommenceMilestoneIndex())
	require.Equal(t, milestone.Index(10), event.StartMilestoneIndex())
	require.Equal(t, milestone.Index(15), event.EndMilestoneIndex())

	env.IssueMilestone() // 5
	require.Equal(t, milestone.Index(5), env.ConfirmedMilestoneIndex())

	env.AssertEventsCount(0, 0)
	env.IssueMilestone() // 6
	env.AssertEventsCount(1, 0)

	return &stakingTestEnv{
		env:     env,
		eventID: eventID,
	}
}

func (s *stakingTestEnv) Cleanup() {
	s.env.Cleanup()
}

func (s *stakingTestEnv) StakeWalletAndIssueMilestone() {
	p := s.env.NewParticipationHelper(s.env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: s.eventID,
			Answers: []byte{},
		}).
		Send()
	s.IssueMilestone(p.Message().StoredMessageID())
}

func (s *stakingTestEnv) StakeWalletThenIncreaseBalanceAndIssueMilestone() {
	p := s.env.NewParticipationHelper(s.env.Wallet1).
		WholeWalletBalance().
		AddParticipation(&participation.Participation{
			EventID: s.eventID,
			Answers: []byte{},
		}).
		Send()

	t := s.env.Transfer(s.env.GenesisWallet, s.env.Wallet1, 1_500_000)
	s.IssueMilestone(p.Message().StoredMessageID(), t.StoredMessageID())
	s.AssertWalletBalance(2_500_000)
}

func (s *stakingTestEnv) CancelStakingAndIssueMilestone() {
	cancelStake := s.env.CancelParticipations(s.env.Wallet1)
	s.IssueMilestone(cancelStake.StoredMessageID())
}

func (s *stakingTestEnv) IncreaseWalletBalanceAndIssueMilestone() {
	transfer := s.env.Transfer(s.env.GenesisWallet, s.env.Wallet1, 1_500_000)
	s.IssueMilestone(transfer.StoredMessageID())
	s.AssertWalletBalance(2_500_000)
}

func (s *stakingTestEnv) IssueMilestone(parents ...hornet.MessageID) {
	s.env.IssueMilestone(parents...)
}

func (s *stakingTestEnv) AssertEventNotCounting() {
	s.env.AssertEventsCount(1, 0)
}

func (s *stakingTestEnv) AssertEventCounting() {
	s.env.AssertEventsCount(1, 1)
}

func (s *stakingTestEnv) AssertEventEnded() {
	s.env.AssertEventsCount(0, 0)
}

func (s *stakingTestEnv) AssertWalletBalance(expected uint64) {
	s.env.AssertWalletBalance(s.env.Wallet1, expected)
}

func (s *stakingTestEnv) AssertWalletRewards(expected uint64) {
	s.env.AssertRewardBalance(s.eventID, s.env.Wallet1.Address(), expected)
}

func (s *stakingTestEnv) AssertWalletRewardsAtIndex(expected uint64, milestoneIndex milestone.Index) {
	s.env.AssertRewardBalance(s.eventID, s.env.Wallet1.Address(), expected, milestoneIndex)
}

func (s *stakingTestEnv) AssertTotalRewards(staked uint64, rewarded uint64) {
	s.env.AssertStakingRewardsStatusAtConfirmedMilestoneIndex(s.eventID, staked, rewarded)
}

func TestStakeCaseA(t *testing.T) {

	// A: (5 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.StakeWalletAndIssueMilestone() // 7
	env.AssertEventNotCounting()
	env.AssertWalletRewards(0)
	env.AssertTotalRewards(1_000_000, 0)
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone() // 11
	env.IssueMilestone() // 12
	env.IssueMilestone() // 13
	env.IssueMilestone() // 14
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewards(5_000_000)
	env.AssertTotalRewards(1_000_000, 5_000_000)

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(4_000_000, 14)
	env.AssertWalletRewardsAtIndex(5_000_000, 15)
}

func TestStakeCaseB(t *testing.T) {

	// B: (3 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 (spent) 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.StakeWalletAndIssueMilestone() // 7
	env.AssertEventNotCounting()
	env.AssertWalletRewards(0)
	env.AssertTotalRewards(1_000_000, 0)
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone()                 // 11
	env.IssueMilestone()                 // 12
	env.IssueMilestone()                 // 13
	env.CancelStakingAndIssueMilestone() // 14
	env.AssertTotalRewards(0, 3_000_000)
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(3_000_000, 14)
	env.AssertWalletRewardsAtIndex(3_000_000, 15)

	env.AssertWalletRewards(3_000_000)
	env.AssertTotalRewards(0, 3_000_000)
}

func TestStakeCaseC(t *testing.T) {

	// C: (3+1 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 (spent) 14 (staked) 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.StakeWalletAndIssueMilestone() // 7
	env.AssertEventNotCounting()
	env.AssertWalletRewards(0)
	env.AssertTotalRewards(1_000_000, 0)
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone()                         // 11
	env.IncreaseWalletBalanceAndIssueMilestone() // 12
	env.IssueMilestone()                         // 13
	env.CancelStakingAndIssueMilestone()         // 14
	env.AssertTotalRewards(0, 3_000_000)
	env.StakeWalletAndIssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(3_000_000, 14)
	env.AssertWalletRewardsAtIndex(5_500_000, 15)

	env.AssertWalletRewards(5_500_000)
	env.AssertTotalRewards(2_500_000, 5_500_000)
}

func TestStakeCaseD(t *testing.T) {

	// D: (3+1 Milestones) = 6 (staked) 7 8 9 10 (event start) 11 12 13 (spent) (staked) 14 (spent) 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.StakeWalletAndIssueMilestone() // 7
	env.AssertEventNotCounting()
	env.AssertWalletRewards(0)
	env.AssertTotalRewards(1_000_000, 0)
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone()                         // 11
	env.IncreaseWalletBalanceAndIssueMilestone() // 12
	env.IssueMilestone()                         // 13
	env.StakeWalletAndIssueMilestone()           // 14
	env.AssertTotalRewards(2_500_000, 5_500_000)
	env.CancelStakingAndIssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(5_500_000, 14)
	env.AssertWalletRewardsAtIndex(5_500_000, 15)

	env.AssertWalletRewards(5_500_000)
	env.AssertTotalRewards(0, 5_500_000)
}

func TestStakeCaseE(t *testing.T) {

	// E: (5 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.StakeWalletAndIssueMilestone() // 11
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.IssueMilestone() // 12
	env.IssueMilestone() // 13
	env.IssueMilestone() // 14
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(4_000_000, 14)
	env.AssertWalletRewardsAtIndex(5_000_000, 15)

	env.AssertWalletRewards(5_000_000)
	env.AssertTotalRewards(1_000_000, 5_000_000)
}

func TestStakeCaseF(t *testing.T) {

	// F: (3 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 (spent) 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.StakeWalletAndIssueMilestone() // 11
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.IssueMilestone()                 // 12
	env.IssueMilestone()                 // 13
	env.CancelStakingAndIssueMilestone() // 14
	env.AssertTotalRewards(0, 3_000_000)
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(3_000_000, 14)
	env.AssertWalletRewardsAtIndex(3_000_000, 15)

	env.AssertWalletRewards(3_000_000)
	env.AssertTotalRewards(0, 3_000_000)
}

func TestStakeCaseG(t *testing.T) {

	// G: (3+2 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 (spent) (staked) 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.StakeWalletAndIssueMilestone() // 11
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.IncreaseWalletBalanceAndIssueMilestone() // 12
	env.IssueMilestone()                         // 13
	env.StakeWalletAndIssueMilestone()           // 14
	env.AssertTotalRewards(2_500_000, 5_500_000)
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(5_500_000, 14)
	env.AssertWalletRewardsAtIndex(8_000_000, 15)

	env.AssertWalletRewards(8_000_000)
	env.AssertTotalRewards(2_500_000, 8_000_000)
}

func TestStakeCaseH(t *testing.T) {

	// H: (3+1 Milestones) = 6 7 8 9 10 (event start) (staked) 11 12 13 (spent) (staked) 14 (spent) 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.StakeWalletAndIssueMilestone() // 11
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.IncreaseWalletBalanceAndIssueMilestone() // 12
	env.IssueMilestone()                         // 13
	env.StakeWalletAndIssueMilestone()           // 14
	env.AssertTotalRewards(2_500_000, 5_500_000)
	env.CancelStakingAndIssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(1_000_000, 11)
	env.AssertWalletRewardsAtIndex(2_000_000, 12)
	env.AssertWalletRewardsAtIndex(3_000_000, 13)
	env.AssertWalletRewardsAtIndex(5_500_000, 14)
	env.AssertWalletRewardsAtIndex(5_500_000, 15)

	env.AssertWalletRewards(5_500_000)
	env.AssertTotalRewards(0, 5_500_000)
}

func TestStakeCaseI(t *testing.T) {

	// I: (3 Milestones) = 6 7 8 9 10 (event start) 11 12 (staked) 13 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone()               // 11
	env.IssueMilestone()               // 12
	env.StakeWalletAndIssueMilestone() // 13
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.IssueMilestone() // 14
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(0, 12)
	env.AssertWalletRewardsAtIndex(1_000_000, 13)
	env.AssertWalletRewardsAtIndex(2_000_000, 14)
	env.AssertWalletRewardsAtIndex(3_000_000, 15)

	env.AssertWalletRewards(3_000_000)
	env.AssertTotalRewards(1_000_000, 3_000_000)
}

func TestStakeCaseJ(t *testing.T) {

	// J: (1 Milestones) = 6 7 8 9 10 (event start) 11 12 (staked) 13 (spent) 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone()               // 11
	env.IssueMilestone()               // 12
	env.StakeWalletAndIssueMilestone() // 13
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.CancelStakingAndIssueMilestone() // 14
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(0, 1_000_000)
	env.IssueMilestone() // 15
	env.AssertEventEnded()
	env.IssueMilestone() // 16

	env.AssertWalletRewardsAtIndex(0, 12)
	env.AssertWalletRewardsAtIndex(1_000_000, 13)
	env.AssertWalletRewardsAtIndex(1_000_000, 14)
	env.AssertWalletRewardsAtIndex(1_000_000, 15)

	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(0, 1_000_000)
}

func TestStakeCaseK(t *testing.T) {

	// K: (1+2 Milestones) = 6 7 8 9 10 (event start) 11 12 (staked) 13 (spent) (staked) 14 15 (event end)

	env := stakingEnv(t)
	defer env.Cleanup()

	env.IssueMilestone() // 7
	env.AssertEventNotCounting()
	env.IssueMilestone() // 8
	env.IssueMilestone() // 9
	env.AssertEventNotCounting()
	env.IssueMilestone() // 10
	env.AssertEventCounting()
	env.IssueMilestone()                                  // 11
	env.IssueMilestone()                                  // 12
	env.StakeWalletThenIncreaseBalanceAndIssueMilestone() // 13
	env.AssertWalletRewards(1_000_000)
	env.AssertTotalRewards(1_000_000, 1_000_000)
	env.StakeWalletAndIssueMilestone() // 14
	env.AssertWalletRewards(3_500_000)
	env.AssertTotalRewards(2_500_000, 3_500_000)
	env.IssueMilestone() // 15
	env.AssertEventEnded()

	env.AssertWalletRewardsAtIndex(0, 12)
	env.AssertWalletRewardsAtIndex(1_000_000, 13)
	env.AssertWalletRewardsAtIndex(3_500_000, 14)
	env.AssertWalletRewardsAtIndex(6_000_000, 15)

	env.AssertWalletRewards(6_000_000)
	env.AssertTotalRewards(2_500_000, 6_000_000)
}

func TestTotalRewards(t *testing.T) {

	participations := []struct {
		amount     uint64
		startIndex milestone.Index
		endIndex   milestone.Index
	}{
		{138706054, 2426818, 2430387},
		{88706054, 2430415, 2664079},
		{199602671, 2664096, 2679429},
		{200000000, 2679439, 2689571},
		{50000000, 2689596, 2712115},
		{5000000, 2712137, 2732909},
		{2500000, 2732925, 2733841},
		{1500000, 2733850, 0},
	}

	assertTotalRewardsFromParticipations(t, participations, 2815845, 110455992)
}

func assertTotalRewardsFromParticipations(t *testing.T, participations []struct {
	amount     uint64
	startIndex milestone.Index
	endIndex   milestone.Index
}, milestoneIndexToCalculate milestone.Index, expectedRewards uint64) {

	env := test.NewParticipationTestEnv(t, 1_000_000, 1_000_000, 1_000_000, 1_000_000, false)
	defer env.Cleanup()

	eventBuilder := participation.NewEventBuilder("AlbinoPugCoin", 2041634, 2102114, 2879714, "The first DogCoin on the Tangle")
	eventBuilder.Payload(&participation.Staking{
		Text:           "The rarest DogCoin on earth",
		Symbol:         "APUG",
		Numerator:      4,
		Denominator:    1000000,
		AdditionalInfo: "Have you seen an albino Pug?",
	})

	event, err := eventBuilder.Build()
	require.NoError(t, err)

	eventID, err := env.ParticipationManager().StoreEvent(event)
	require.NoError(t, err)

	var sum uint64
	for _, p := range participations {
		rewards, err := env.ParticipationManager().RewardsForTrackedParticipation(&participation.TrackedParticipation{
			EventID:    eventID,
			OutputID:   &iotago.OutputID{},
			MessageID:  hornet.NullMessageID(),
			Amount:     p.amount,
			StartIndex: p.startIndex,
			EndIndex:   p.endIndex,
		}, milestoneIndexToCalculate)
		require.NoError(t, err)
		sum += rewards
	}

	require.Equal(t, int(expectedRewards), int(sum)) // converting to int to have a readable test output
}
