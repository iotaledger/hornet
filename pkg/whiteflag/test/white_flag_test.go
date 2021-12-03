package test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	seed1, _ = hex.DecodeString("96d9ff7a79e4b0a5f3e5848ae7867064402da92a62eabb4ebbe463f12d1f3b1aace1775488f51cb1e3a80732a03ef60b111d6833ab605aa9f8faebeb33bbe3d9")
	seed2, _ = hex.DecodeString("b15209ddc93cbdb600137ea6a8f88cdd7c5d480d5815c9352a0fb5c4e4b86f7151dcb44c2ba635657a2df5a8fd48cb9bab674a9eceea527dbbb254ef8c9f9cd7")
	seed3, _ = hex.DecodeString("d5353ceeed380ab89a0f6abe4630c2091acc82617c0edd4ff10bd60bba89e2ed30805ef095b989c2bf208a474f8748d11d954aade374380422d4d812b6f1da90")
	seed4, _ = hex.DecodeString("bd6fe09d8a309ca309c5db7b63513240490109cd0ac6b123551e9da0d5c8916c4a5a4f817e4b4e9df89885ce1af0986da9f1e56b65153c2af1e87ab3b11dabb4")

	showConfirmationGraphs = false
	MinPoWScore            = 100.0
	BelowMaxDepth          = 15
)

func TestWhiteFlagSendAllCoins(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, BelowMaxDepth, MinPoWScore, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, iotago.TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	messageA := te.NewMessageBuilder("A").
		Parents(hornet.MessageIDs{te.Milestones[0].Milestone().MessageID, te.Milestones[1].Milestone().MessageID}).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(iotago.TokenSupply).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at message A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageA.StoredMessageID()}, true)
	require.Equal(t, 1+1, confStats.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 0)
	te.AssertWalletBalance(seed2Wallet, iotago.TokenSupply)

	// Issue some transactions
	messageB := te.NewMessageBuilder("B").
		Parents(hornet.MessageIDs{messageA.StoredMessageID(), te.Milestones[2].Milestone().MessageID}).
		FromWallet(seed2Wallet).
		ToWallet(seed1Wallet).
		Amount(iotago.TokenSupply).
		Build().
		Store().
		BookOnWallets()

	// Confirming milestone at message C (message D and E are not included)
	_, confStats = te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageB.StoredMessageID()}, true)
	require.Equal(t, 1+1, confStats.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, iotago.TokenSupply)
	te.AssertWalletBalance(seed2Wallet, 0)
}

func TestWhiteFlagWithMultipleConflicting(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)
	seed4Wallet := utils.NewHDWallet("Seed4", seed4, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, BelowMaxDepth, MinPoWScore, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, iotago.TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	// Valid transfer from seed1 (iotago.TokenSupply) with remainder seed1 (2_779_530_282_277_761) to seed2 (1_000_000)
	messageA := te.NewMessageBuilder("A").
		Parents(hornet.MessageIDs{te.Milestones[0].Milestone().MessageID, te.Milestones[1].Milestone().MessageID}).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1_000_000).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Valid transfer from seed1 (2_779_530_282_277_761) with remainder seed1 (2_779_530_280_277_761) to seed2 (2_000_000)
	messageB := te.NewMessageBuilder("B").
		Parents(hornet.MessageIDs{messageA.StoredMessageID(), te.Milestones[0].Milestone().MessageID}).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(2_000_000).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Invalid transfer from seed3 (0) to seed2 (100_000) (invalid input)
	messageC := te.NewMessageBuilder("C").
		Parents(hornet.MessageIDs{te.Milestones[2].Milestone().MessageID, messageB.StoredMessageID()}).
		FromWallet(seed3Wallet).
		ToWallet(seed2Wallet).
		Amount(100_000).
		FakeInputs().
		Build().
		Store()

	// Confirming milestone at message C (message D and E are not included)
	_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageC.StoredMessageID()}, true)
	require.Equal(t, 3+1, confStats.MessagesReferenced) // 3 + milestone itself
	require.Equal(t, 2, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageC.StoredMessageID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 3_000_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 0)

	// Invalid transfer from seed4 (0) to seed2 (1_500_000) (invalid input)
	messageD := te.NewMessageBuilder("D").
		Parents(hornet.MessageIDs{messageA.StoredMessageID(), messageC.StoredMessageID()}).
		FromWallet(seed4Wallet).
		ToWallet(seed2Wallet).
		Amount(1_500_000).
		FakeInputs().
		Build().
		Store()

	// Valid transfer from seed2 (1_000_000) and seed2 (2_000_000) with remainder seed2 (1_500_000) to seed4 (1_500_000)
	messageE := te.NewMessageBuilder("E").
		Parents(hornet.MessageIDs{messageB.StoredMessageID(), messageD.StoredMessageID()}).
		FromWallet(seed2Wallet).
		ToWallet(seed4Wallet).
		Amount(1_500_000).
		Build().
		Store().
		BookOnWallets()

	seed4WalletOutput := messageE.GeneratedUTXO()

	seed2Wallet.PrintStatus()
	seed4Wallet.PrintStatus()

	// Confirming milestone at message E
	_, confStats = te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageE.StoredMessageID()}, true)
	require.Equal(t, 2+1, confStats.MessagesReferenced) // 2 + milestone itself
	require.Equal(t, 1, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageD.StoredMessageID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 1_500_000)

	// Invalid transfer from seed3 (0) to seed2 (100_000) (already spent (genesis))
	messageF := te.NewMessageBuilder("F").
		Parents(hornet.MessageIDs{te.Milestones[3].Milestone().MessageID, messageE.StoredMessageID()}).
		FromWallet(seed3Wallet).
		ToWallet(seed2Wallet).
		Amount(100_000).
		UsingOutput(te.GenesisOutput).
		Build().
		Store()

	// Confirming milestone at message F
	_, confStats = te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageF.StoredMessageID()}, true)
	require.Equal(t, 1+1, confStats.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 0, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageF.StoredMessageID(), storage.ConflictInputUTXOAlreadySpent)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 1_500_000)

	// Valid transfer from seed4 to seed3 (1_500_000)
	messageG := te.NewMessageBuilder("G").
		Parents(hornet.MessageIDs{te.Milestones[4].Milestone().MessageID, messageF.StoredMessageID()}).
		FromWallet(seed4Wallet).
		ToWallet(seed3Wallet).
		Amount(1_500_000).
		UsingOutput(seed4WalletOutput).
		Build().
		Store().
		BookOnWallets()

	// Valid transfer from seed4 to seed2 (1_500_000) (double spend -> already spent)
	messageH := te.NewMessageBuilder("H").
		Parents(hornet.MessageIDs{messageG.StoredMessageID()}).
		FromWallet(seed4Wallet).
		ToWallet(seed2Wallet).
		Amount(1_500_000).
		UsingOutput(seed4WalletOutput).
		Build().
		Store()

	// Confirming milestone at message H
	_, confStats = te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageH.StoredMessageID()}, true)
	require.Equal(t, 2+1, confStats.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageH.StoredMessageID(), storage.ConflictInputUTXOAlreadySpentInThisMilestone)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 1_500_000)
	te.AssertWalletBalance(seed4Wallet, 0)
}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	genesisWallet := utils.NewHDWallet("Seed1", seed1, 0)
	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 3, BelowMaxDepth, MinPoWScore, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	genesisWallet.BookOutput(te.GenesisOutput)

	// Issue some transactions
	messageA := te.NewMessageBuilder("A").Parents(hornet.MessageIDs{te.Milestones[0].Milestone().MessageID, te.Milestones[1].Milestone().MessageID}).BuildIndexation().Store()
	messageB := te.NewMessageBuilder("B").Parents(hornet.MessageIDs{messageA.StoredMessageID(), te.Milestones[0].Milestone().MessageID}).BuildIndexation().Store()
	messageC := te.NewMessageBuilder("C").Parents(hornet.MessageIDs{te.Milestones[2].Milestone().MessageID, te.Milestones[0].Milestone().MessageID}).BuildIndexation().Store()
	messageD := te.NewMessageBuilder("D").Parents(hornet.MessageIDs{messageB.StoredMessageID(), messageC.StoredMessageID()}).BuildIndexation().Store()
	messageE := te.NewMessageBuilder("E").Parents(hornet.MessageIDs{messageB.StoredMessageID(), messageA.StoredMessageID()}).BuildIndexation().Store()

	// Confirming milestone include all msg up to message E. This should only include A, B and E
	_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageE.StoredMessageID()}, true)
	require.Equal(t, 3+1, confStats.MessagesReferenced) // A, B, E + 1 for Milestone
	require.Equal(t, 0, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, confStats.MessagesExcludedWithoutTransactions) // 1 is for the milestone itself

	// Issue another message
	messageF := te.NewMessageBuilder("F").Parents(hornet.MessageIDs{messageD.StoredMessageID(), messageE.StoredMessageID()}).BuildIndexation().Store()

	// Confirming milestone at message F. This should confirm D, C and F
	_, confStats = te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{messageF.StoredMessageID()}, true)

	require.Equal(t, 3+1, confStats.MessagesReferenced) // D, C, F + 1 for Milestone
	require.Equal(t, 0, confStats.MessagesIncludedWithTransactions)
	require.Equal(t, 0, confStats.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, confStats.MessagesExcludedWithoutTransactions) // 1 is for the milestone itself
}
