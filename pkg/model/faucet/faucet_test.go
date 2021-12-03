package faucet_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v3"

	"github.com/gohornet/hornet/pkg/model/faucet/test"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

func TestSingleRequest(t *testing.T) {
	// requests to a single address

	var faucetBalance uint64 = 1_000_000_000        //  1 Gi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	err := env.RequestFundsAndIssueMilestone(env.Wallet1)
	require.NoError(t, err)

	faucetBalance -= faucetAmount
	calculatedWallet1Balance := wallet1Balance + faucetAmount
	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)

	// small amount
	for calculatedWallet1Balance < faucetMaxAddressBalance {
		err = env.RequestFundsAndIssueMilestone(env.Wallet1)
		require.NoError(t, err)

		faucetBalance -= faucetSmallAmount
		calculatedWallet1Balance += faucetSmallAmount
		env.AssertFaucetBalance(faucetBalance)
		env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
		env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)
	}

	// max reached
	err = env.RequestFundsAndIssueMilestone(env.Wallet1)
	require.Error(t, err)
}

func TestMultipleRequests(t *testing.T) {
	// requests to multiple addresses

	var faucetBalance uint64 = 1_000_000_000        //  1 Gi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	// multiple target addresses in single messages
	tips1, err := env.RequestFunds(env.Wallet1)
	require.NoError(t, err)

	tips2, err := env.RequestFunds(env.Wallet2)
	require.NoError(t, err)

	tips3, err := env.RequestFunds(env.Wallet3)
	require.NoError(t, err)

	var tips hornet.MessageIDs
	tips = append(tips, tips1...)
	tips = append(tips, tips2...)
	tips = append(tips, tips3...)
	_, _ = env.IssueMilestone(tips...)

	faucetBalance -= 3 * faucetAmount
	calculatedWallet1Balance := wallet1Balance + faucetAmount
	calculatedWallet2Balance := wallet2Balance + faucetAmount
	calculatedWallet3Balance := wallet3Balance + faucetAmount
	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, calculatedWallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, calculatedWallet3Balance)

	// small amount
	for calculatedWallet1Balance < faucetMaxAddressBalance {
		// multiple target addresses in one message
		err = env.RequestFundsAndIssueMilestone(env.Wallet1, env.Wallet2, env.Wallet3)
		require.NoError(t, err)

		faucetBalance -= 3 * faucetSmallAmount
		calculatedWallet1Balance += faucetSmallAmount
		calculatedWallet2Balance += faucetSmallAmount
		calculatedWallet3Balance += faucetSmallAmount
		env.AssertFaucetBalance(faucetBalance)
		env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
		env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)
		env.TestEnv.AssertLedgerBalance(env.Wallet2, calculatedWallet2Balance)
		env.TestEnv.AssertLedgerBalance(env.Wallet3, calculatedWallet3Balance)
	}

	// max reached
	err = env.RequestFundsAndIssueMilestone(env.Wallet1, env.Wallet2, env.Wallet3)
	require.Error(t, err)
}

func TestDoubleSpent(t *testing.T) {
	// reuse of the private key of the faucet (double spent)

	var faucetBalance uint64 = 1_000_000_000        //  1 Gi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	// create a conflicting transaction that gets confirmed instead of the faucet message
	message := env.TestEnv.NewMessageBuilder().
		LatestMilestonesAsParents().
		FromWallet(env.FaucetWallet).
		ToWallet(env.GenesisWallet).
		Amount(faucetAmount).
		Build().
		Store().
		BookOnWallets()

	// create the confliction message in the faucet
	tips, err := env.RequestFunds(env.Wallet1)
	require.NoError(t, err)

	// Confirming milestone at message
	_, _ = env.IssueMilestone(message.StoredMessageID())

	genesisBalance += faucetAmount
	faucetBalance -= faucetAmount                         // we stole some funds from the faucet
	env.AssertFaucetBalance(faucetBalance - faucetAmount) // pending request is also taken into account
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)

	// Confirming milestone at message (double spent)
	_, _ = env.IssueMilestone(tips...)

	env.AssertFaucetBalance(faucetBalance - faucetAmount) // request is still pending
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)

	err = env.FlushRequestsAndConfirmNewFaucetMessage()
	require.NoError(t, err)

	faucetBalance -= faucetAmount // now the request is booked
	calculatedWallet1Balance := wallet1Balance + faucetAmount
	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)
}

func TestBelowMaxDepth(t *testing.T) {
	// faucet message is below max depth and never confirmed

	var faucetBalance uint64 = 1_000_000_000        //  1 Gi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	// create a request that doesn't get confirmed
	_, err := env.RequestFunds(env.Wallet1)
	require.NoError(t, err)

	// issue several milestones, so that the faucet message gets below max depth
	for i := 0; i <= test.BelowMaxDepth; i++ {
		_, _ = env.IssueMilestone()
	}

	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)

	// flushing requests should reissue the requests that were in the below max depth message
	err = env.FlushRequestsAndConfirmNewFaucetMessage()
	require.NoError(t, err)

	calculatedWallet1Balance := wallet1Balance + faucetAmount
	faucetBalance -= faucetAmount
	env.AssertFaucetBalance(faucetBalance)

	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)
}

func TestBelowMaxDepthAfterRequest(t *testing.T) {
	// first a faucet message is confirmed, but then the old message
	// was used as a tip for the next faucet message
	// which caused that it is below max depth and never confirmed

	var faucetBalance uint64 = 1_000_000_000        //  1 Gi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	err := env.RequestFundsAndIssueMilestone(env.Wallet1)
	require.NoError(t, err)

	faucetBalance -= faucetAmount
	calculatedWallet1Balance := wallet1Balance + faucetAmount
	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)

	// issue several milestones, so that the faucet message gets below max depth
	for i := 0; i <= test.BelowMaxDepth; i++ {
		_, _ = env.IssueMilestone()
	}

	err = env.RequestFundsAndIssueMilestone(env.Wallet1)
	require.NoError(t, err)

	faucetBalance -= faucetSmallAmount
	calculatedWallet1Balance += faucetSmallAmount
	env.AssertFaucetBalance(faucetBalance)

	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)
}

func TestNotEnoughFaucetFunds(t *testing.T) {
	// check if faucet returns an error if not enough funds available

	var faucetBalance uint64 = 29000000             // 29 Mi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	// 29 Mi - 10 Mi = 19 Mi
	err := env.RequestFundsAndIssueMilestone(env.Wallet1)
	require.NoError(t, err)

	faucetBalance -= faucetAmount
	env.AssertFaucetBalance(faucetBalance)

	// 19 Mi - 10 Mi = 9 Mi
	err = env.RequestFundsAndIssueMilestone(env.Wallet2)
	require.NoError(t, err)

	faucetBalance -= faucetAmount
	env.AssertFaucetBalance(faucetBalance)

	// 9 Mi - 10 Mi = error
	err = env.RequestFundsAndIssueMilestone(env.Wallet3)
	require.Error(t, err)

	env.AssertFaucetBalance(faucetBalance)
}

func TestCollectFaucetFunds(t *testing.T) {
	// check if faucet collects funds if no requests left

	var faucetBalance uint64 = 1_000_000_000        //  1 Gi
	var wallet1Balance uint64 = 0                   //  0  i
	var wallet2Balance uint64 = 0                   //  0  i
	var wallet3Balance uint64 = 0                   //  0  i
	var faucetAmount uint64 = 10_000_000            // 10 Mi
	var faucetSmallAmount uint64 = 1_000_000        //  1 Mi
	var faucetMaxAddressBalance uint64 = 20_000_000 // 20 Mi

	env := test.NewFaucetTestEnv(t,
		faucetBalance,
		wallet1Balance,
		wallet2Balance,
		wallet3Balance,
		faucetAmount,
		faucetSmallAmount,
		faucetMaxAddressBalance,
		false)
	defer env.Cleanup()
	require.NotNil(t, env)

	confirmedMilestoneIndex := env.ConfirmedMilestoneIndex() // 4
	require.Equal(t, milestone.Index(4), confirmedMilestoneIndex)

	// Verify balances
	genesisBalance := iotago.TokenSupply - faucetBalance - wallet1Balance - wallet2Balance - wallet3Balance

	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.GenesisWallet, genesisBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, wallet1Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet2, wallet2Balance)
	env.TestEnv.AssertLedgerBalance(env.Wallet3, wallet3Balance)

	env.AssertAddressUTXOCount(env.FaucetWallet.Address(), 1)

	err := env.RequestFundsAndIssueMilestone(env.Wallet1)
	require.NoError(t, err)

	env.AssertAddressUTXOCount(env.FaucetWallet.Address(), 1)

	faucetBalance -= faucetAmount
	calculatedWallet1Balance := wallet1Balance + faucetAmount
	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.Wallet1, calculatedWallet1Balance)

	message := env.TestEnv.NewMessageBuilder().
		LatestMilestonesAsParents().
		FromWallet(env.GenesisWallet).
		ToWallet(env.FaucetWallet).
		Amount(faucetAmount).
		Build().
		Store().
		BookOnWallets()

	// Confirming milestone at message
	_, _ = env.IssueMilestone(message.StoredMessageID())

	faucetBalance += faucetAmount
	env.AssertFaucetBalance(faucetBalance)
	env.TestEnv.AssertLedgerBalance(env.FaucetWallet, faucetBalance)

	env.AssertAddressUTXOCount(env.FaucetWallet.Address(), 2)

	// Flushing requests should collect all outputs
	err = env.FlushRequestsAndConfirmNewFaucetMessage()
	require.NoError(t, err)

	env.AssertAddressUTXOCount(env.FaucetWallet.Address(), 1)
}
