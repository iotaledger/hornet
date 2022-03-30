package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/faucet"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/pow"
)

const (
	faucetMaxOutputCount = iotago.MaxOutputsCount
	faucetTagMessage     = "FAUCET"
	faucetPowWorkerCount = 0
	faucetBatchTimeout   = 2 * time.Second
)

var (
	genesisSeed, _ = hex.DecodeString("2f54b071657e6644629a40518ba6554de4eee89f0757713005ad26137d80968d05e1ca1bca555d8b4b85a3f4fcf11a6a48d3d628d1ace40f48009704472fc8f9")
	faucetSeed, _  = hex.DecodeString("96d9ff7a79e4b0a5f3e5848ae7867064402da92a62eabb4ebbe463f12d1f3b1aace1775488f51cb1e3a80732a03ef60b111d6833ab605aa9f8faebeb33bbe3d9")
	seed1, _       = hex.DecodeString("b15209ddc93cbdb600137ea6a8f88cdd7c5d480d5815c9352a0fb5c4e4b86f7151dcb44c2ba635657a2df5a8fd48cb9bab674a9eceea527dbbb254ef8c9f9cd7")
	seed2, _       = hex.DecodeString("d5353ceeed380ab89a0f6abe4630c2091acc82617c0edd4ff10bd60bba89e2ed30805ef095b989c2bf208a474f8748d11d954aade374380422d4d812b6f1da90")
	seed3, _       = hex.DecodeString("bd6fe09d8a309ca309c5db7b63513240490109cd0ac6b123551e9da0d5c8916c4a5a4f817e4b4e9df89885ce1af0986da9f1e56b65153c2af1e87ab3b11dabb4")

	MinPoWScore   = 10.0
	BelowMaxDepth = 15
)

type FaucetTestEnv struct {
	t       *testing.T
	TestEnv *testsuite.TestEnvironment

	GenesisWallet *utils.HDWallet
	FaucetWallet  *utils.HDWallet
	Wallet1       *utils.HDWallet
	Wallet2       *utils.HDWallet
	Wallet3       *utils.HDWallet

	Faucet *faucet.Faucet

	faucetCtxCancel context.CancelFunc
}

func NewFaucetTestEnv(t *testing.T,
	faucetBalance uint64,
	wallet1Balance uint64,
	wallet2Balance uint64,
	wallet3Balance uint64,
	faucetAmount uint64,
	faucetSmallAmount uint64,
	faucetMaxAddressBalance uint64,
	assertSteps bool) *FaucetTestEnv {

	genesisWallet := utils.NewHDWallet("Genesis", genesisSeed, 0)
	faucetWallet := utils.NewHDWallet("Faucet", faucetSeed, 0)
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)

	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, BelowMaxDepth, MinPoWScore, false)

	// Add token supply to our local HDWallet
	genesisWallet.BookOutput(te.GenesisOutput)
	if assertSteps {
		te.AssertWalletBalance(genesisWallet, iotago.TokenSupply)
	}

	lastMessageID := te.Milestones[0].Milestone().MessageID
	messagesCount := 0

	// Fund Faucet
	if faucetBalance > 0 {
		messageA := te.NewMessageBuilder("A").
			Parents(hornet.MessageIDs{lastMessageID, te.Milestones[1].Milestone().MessageID}).
			FromWallet(genesisWallet).
			ToWallet(faucetWallet).
			Amount(faucetBalance).
			Build().
			Store().
			BookOnWallets()

		messagesCount++
		lastMessageID = messageA.StoredMessageID()
	}

	// Fund Wallet1
	if wallet1Balance > 0 {
		messageB := te.NewMessageBuilder("B").
			Parents(hornet.MessageIDs{lastMessageID, te.Milestones[1].Milestone().MessageID}).
			FromWallet(genesisWallet).
			ToWallet(seed1Wallet).
			Amount(wallet1Balance).
			Build().
			Store().
			BookOnWallets()

		messagesCount++
		lastMessageID = messageB.StoredMessageID()
	}

	// Fund Wallet2
	if wallet2Balance > 0 {
		messageC := te.NewMessageBuilder("C").
			Parents(hornet.MessageIDs{lastMessageID, te.Milestones[1].Milestone().MessageID}).
			FromWallet(genesisWallet).
			ToWallet(seed2Wallet).
			Amount(wallet2Balance).
			Build().
			Store().
			BookOnWallets()

		messagesCount++
		lastMessageID = messageC.StoredMessageID()

	}

	// Fund Wallet3
	if wallet3Balance > 0 {
		messageD := te.NewMessageBuilder("D").
			Parents(hornet.MessageIDs{lastMessageID, te.Milestones[1].Milestone().MessageID}).
			FromWallet(genesisWallet).
			ToWallet(seed3Wallet).
			Amount(wallet3Balance).
			Build().
			Store().
			BookOnWallets()

		messagesCount++
		lastMessageID = messageD.StoredMessageID()

	}

	// Confirming milestone at message D
	_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{lastMessageID}, false)
	if assertSteps {

		require.Equal(t, messagesCount+1, confStats.MessagesReferenced) // messagesCount + milestone itself
		require.Equal(t, messagesCount, confStats.MessagesIncludedWithTransactions)
		require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
		require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

		// Verify balances
		te.AssertWalletBalance(genesisWallet, iotago.TokenSupply-faucetBalance-wallet1Balance-wallet2Balance-wallet3Balance)
		te.AssertWalletBalance(faucetWallet, faucetBalance)
		te.AssertWalletBalance(seed1Wallet, wallet1Balance)
		te.AssertWalletBalance(seed2Wallet, wallet2Balance)
		te.AssertWalletBalance(seed3Wallet, wallet3Balance)
	}

	defaultDaemon := daemon.New()
	defaultDaemon.Start()

	var tipselFunc faucet.TipselFunc = func() (tips hornet.MessageIDs, err error) {
		// issue all faucet messages on the latest milestone
		return hornet.MessageIDs{te.LastMilestoneMessageID}, nil
	}

	storeMessageFunc := func(msg *storage.Message) error {

		if msg.ProtocolVersion() != iotago.ProtocolVersion {
			return fmt.Errorf("msg has invalid protocol version %d instead of %d", msg.ProtocolVersion(), iotago.ProtocolVersion)
		}

		score := pow.Score(msg.Data())
		if score < MinPoWScore {
			return fmt.Errorf("msg has insufficient PoW score %0.2f", score)
		}

		cmi := te.SyncManager().ConfirmedMilestoneIndex()

		checkParentFunc := func(messageID hornet.MessageID) error {
			cachedMsgMeta := te.Storage().CachedMessageMetadataOrNil(messageID) // meta +1
			if cachedMsgMeta == nil {
				// parent not found
				entryPointIndex, exists, err := te.Storage().SolidEntryPointsIndex(messageID)
				if err != nil {
					return err
				}
				if !exists {
					return gossip.ErrMessageNotSolid
				}

				if (cmi - entryPointIndex) > milestone.Index(BelowMaxDepth) {
					// the parent is below max depth
					return gossip.ErrMessageBelowMaxDepth
				}

				// message is a SEP and not below max depth
				return nil
			}
			defer cachedMsgMeta.Release(true) // meta -1

			if !cachedMsgMeta.Metadata().IsSolid() {
				// if the parent is not solid, the message itself can't be solid
				return gossip.ErrMessageNotSolid
			}

			// we pass a background context here to not prevent emitting messages at shutdown (COO etc).
			_, ocri, err := dag.ConeRootIndexes(context.Background(), te.Storage(), cachedMsgMeta.Retain(), cmi) // meta pass +1
			if err != nil {
				return err
			}

			if (cmi - ocri) > milestone.Index(BelowMaxDepth) {
				// the parent is below max depth
				return gossip.ErrMessageBelowMaxDepth
			}

			return nil
		}

		for _, parentMsgID := range msg.Parents() {
			err := checkParentFunc(parentMsgID)
			if err != nil {
				return err
			}
		}

		_ = te.StoreMessage(msg) // no need to release, since we remember all the messages for later cleanup

		return nil
	}

	f := faucet.New(
		defaultDaemon,
		te.Storage(),
		te.SyncManager(),
		te.NetworkID(),
		testsuite.DeSerializationParameters,
		int(te.BelowMaxDepth()),
		te.UTXOManager(),
		faucetWallet.Address(),
		faucetWallet.AddressSigner(),
		tipselFunc,
		te.PoWHandler,
		storeMessageFunc,
		faucet.WithHRPNetworkPrefix(iotago.PrefixTestnet),
		faucet.WithAmount(faucetAmount),
		faucet.WithSmallAmount(faucetSmallAmount),
		faucet.WithMaxAddressBalance(faucetMaxAddressBalance),
		faucet.WithMaxOutputCount(faucetMaxOutputCount),
		faucet.WithTagMessage(faucetTagMessage),
		faucet.WithBatchTimeout(faucetBatchTimeout),
		faucet.WithPowWorkerCount(faucetPowWorkerCount),
	)

	faucetCtx, faucetCtxCancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		if err := f.RunFaucetLoop(faucetCtx, func() {
			wg.Done()
		}); err != nil && common.IsCriticalError(err) != nil {
			require.NoError(t, err)
		}
	}()

	// wait until faucet is initialized
	wg.Wait()

	// Connect the callbacks from the testsuite to the Faucet
	te.ConfigureUTXOCallbacks(
		nil,
		nil,
		func(confirmation *whiteflag.Confirmation) {
			require.NoError(t, f.ApplyConfirmation(confirmation))
		},
		nil,
		nil,
	)

	return &FaucetTestEnv{
		t:               t,
		TestEnv:         te,
		GenesisWallet:   genesisWallet,
		FaucetWallet:    faucetWallet,
		Wallet1:         seed1Wallet,
		Wallet2:         seed2Wallet,
		Wallet3:         seed3Wallet,
		Faucet:          f,
		faucetCtxCancel: faucetCtxCancel,
	}
}

func (env *FaucetTestEnv) ConfirmedMilestoneIndex() milestone.Index {
	return env.TestEnv.SyncManager().ConfirmedMilestoneIndex()
}

func (env *FaucetTestEnv) Cleanup() {
	if env.faucetCtxCancel != nil {
		env.faucetCtxCancel()
	}
	env.TestEnv.CleanupTestEnvironment(true)
}

func (env *FaucetTestEnv) processFaucetRequests(preFlushFunc func() error) (hornet.MessageIDs, error) {

	wg := sync.WaitGroup{}
	wg.Add(1)

	var tips hornet.MessageIDs
	onFaucetIssuedMessage := events.NewClosure(func(messageID hornet.MessageID) {
		tips = append(tips, messageID)
		wg.Done()
	})
	env.Faucet.Events.IssuedMessage.Attach(onFaucetIssuedMessage)
	defer env.Faucet.Events.IssuedMessage.Detach(onFaucetIssuedMessage)

	if preFlushFunc != nil {
		if err := preFlushFunc(); err != nil {
			return nil, err
		}
	}

	env.Faucet.FlushRequests()

	chanDone := make(chan struct{})

	go func() {
		wg.Wait()
		close(chanDone)
	}()

	select {
	case <-chanDone:
	case <-time.After(1 * time.Second):
		env.t.Error("attachment of faucet message took too long")
	}

	return tips, nil
}

// RequestFunds sends requests to the faucet and waits until the next faucet message is issued.
func (env *FaucetTestEnv) RequestFunds(wallets ...*utils.HDWallet) (hornet.MessageIDs, error) {

	require.Greater(env.t, len(wallets), 0)

	tips, err := env.processFaucetRequests(func() error {
		for _, wallet := range wallets {
			if _, err := env.Faucet.Enqueue(wallet.Address().Bech32(iotago.PrefixTestnet)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return tips, nil
}

// RequestFundsAndIssueMilestone sends requests to the faucet, waits until the next faucet message is issued and
// issues a milestone on top of it.
func (env *FaucetTestEnv) RequestFundsAndIssueMilestone(wallets ...*utils.HDWallet) error {

	tips, err := env.RequestFunds(wallets...)
	if err != nil {
		return err
	}

	// issue milestone on top of new faucet message
	_, _ = env.IssueMilestone(tips...)
	return nil
}

// FlushRequestsAndConfirmNewFaucetMessage flushes pending faucet requests, waits until the next faucet message is issued and
// issues a milestone on top of it.
func (env *FaucetTestEnv) FlushRequestsAndConfirmNewFaucetMessage() error {

	tips, err := env.processFaucetRequests(nil)
	if err != nil {
		return err
	}

	// issue milestone on top of new faucet message
	_, _ = env.IssueMilestone(tips...)
	return nil
}

func (env *FaucetTestEnv) IssueMilestone(onTips ...hornet.MessageID) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {
	if len(onTips) == 0 {
		return env.TestEnv.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{env.TestEnv.LastMilestoneMessageID}, false)
	}
	return env.TestEnv.IssueAndConfirmMilestoneOnTips(onTips, false)
}

func (env *FaucetTestEnv) AssertFaucetBalance(expected uint64) {
	faucetInfo, err := env.Faucet.Info()
	require.NoError(env.t, err)
	require.Exactly(env.t, expected, faucetInfo.Balance)
}

func (env *FaucetTestEnv) AssertAddressUTXOCount(address iotago.Address, expected int) {
	_, count, err := env.TestEnv.ComputeAddressBalanceWithoutConstraints(address)
	require.NoError(env.t, err)
	require.Equal(env.t, expected, count)
}
