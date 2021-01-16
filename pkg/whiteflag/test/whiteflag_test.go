package test

import (
	"encoding/hex"
	"testing"

	_ "golang.org/x/crypto/blake2b"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
)

var (
	seed1, _ = hex.DecodeString("96d9ff7a79e4b0a5f3e5848ae7867064402da92a62eabb4ebbe463f12d1f3b1aace1775488f51cb1e3a80732a03ef60b111d6833ab605aa9f8faebeb33bbe3d9")
	seed2, _ = hex.DecodeString("b15209ddc93cbdb600137ea6a8f88cdd7c5d480d5815c9352a0fb5c4e4b86f7151dcb44c2ba635657a2df5a8fd48cb9bab674a9eceea527dbbb254ef8c9f9cd7")
	seed3, _ = hex.DecodeString("d5353ceeed380ab89a0f6abe4630c2091acc82617c0edd4ff10bd60bba89e2ed30805ef095b989c2bf208a474f8748d11d954aade374380422d4d812b6f1da90")
	seed4, _ = hex.DecodeString("bd6fe09d8a309ca309c5db7b63513240490109cd0ac6b123551e9da0d5c8916c4a5a4f817e4b4e9df89885ce1af0986da9f1e56b65153c2af1e87ab3b11dabb4")

	showConfirmationGraphs = false
)

func TestWhiteFlagWithMultipleConflicting(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)
	seed4Wallet := utils.NewHDWallet("Seed4", seed4, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, 2_779_530_283_277_761)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	// Valid transfer from seed1 (2_779_530_283_277_761) with remainder seed1 (2_779_530_282_277_761) to seed2 (1_000_000)
	messageA, messageAConsumedOutputs, messageASentOutput, messageARemainderOutput := utils.MsgWithValueTx(t,
		te.Milestones[0].GetMilestone().MessageID,
		te.Milestones[1].GetMilestone().MessageID,
		"A",
		seed1Wallet,
		seed2Wallet,
		1_000_000,
		te.PowHandler,
	)
	cachedMessageA := te.StoreMessage(messageA)
	seed1Wallet.BookSpents(messageAConsumedOutputs)
	seed2Wallet.BookOutput(messageASentOutput)
	seed1Wallet.BookOutput(messageARemainderOutput)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Valid transfer from seed1 (2_779_530_282_277_761) with remainder seed1 (2_779_530_280_277_761) to seed2 (2_000_000)
	messageB, messageBConsumedOutputs, messageBSentOutput, messageBRemainderOutput := utils.MsgWithValueTx(t,
		cachedMessageA.GetMessage().GetMessageID(),
		te.Milestones[0].GetMilestone().MessageID,
		"B",
		seed1Wallet,
		seed2Wallet,
		2_000_000,
		te.PowHandler,
	)
	cachedMessageB := te.StoreMessage(messageB)
	seed1Wallet.BookSpents(messageBConsumedOutputs)
	seed2Wallet.BookOutput(messageBSentOutput)
	seed1Wallet.BookOutput(messageBRemainderOutput)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Invalid transfer from seed3 (0) to seed2 (100_000) (invalid input)
	messageC, _, _, _ := utils.MsgWithInvalidValueTx(t,
		te.Milestones[2].GetMilestone().MessageID,
		cachedMessageB.GetMessage().GetMessageID(),
		"C",
		seed3Wallet,
		seed2Wallet,
		100_000,
		te.PowHandler,
	)
	cachedMessageC := te.StoreMessage(messageC)

	// Confirming milestone at message C (message D and E are not included)
	conf := te.IssueAndConfirmMilestoneOnTip(cachedMessageC.GetMessage().GetMessageID(), true)

	require.Equal(t, 3+1, conf.MessagesReferenced) // 3 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageC.GetMessageID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 3_000_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 0)

	// Invalid transfer from seed4 (0) to seed2 (1_500_000) (invalid input)
	messageD, _, _, _ := utils.MsgWithInvalidValueTx(t,
		cachedMessageA.GetMessage().GetMessageID(),
		cachedMessageC.GetMessage().GetMessageID(),
		"D",
		seed4Wallet,
		seed2Wallet,
		1_500_000,
		te.PowHandler,
	)
	cachedMessageD := te.StoreMessage(messageD)

	// Valid transfer from seed2 (1_000_000) and seed2 (2_000_000) with remainder seed2 (1_500_000) to seed4 (1_500_000)
	messageE, messageEConsumedOutputs, messageESentOutput, messageERemainderOutput := utils.MsgWithValueTx(t,
		cachedMessageB.GetMessage().GetMessageID(),
		cachedMessageD.GetMessage().GetMessageID(),
		"E",
		seed2Wallet,
		seed4Wallet,
		1_500_000,
		te.PowHandler,
	)
	cachedMessageE := te.StoreMessage(messageE)
	seed2Wallet.BookSpents(messageEConsumedOutputs)
	seed4Wallet.BookOutput(messageESentOutput)
	seed2Wallet.BookOutput(messageERemainderOutput)

	seed2Wallet.PrintStatus()
	seed4Wallet.PrintStatus()

	// Confirming milestone at message E
	conf = te.IssueAndConfirmMilestoneOnTip(cachedMessageE.GetMessage().GetMessageID(), true)
	require.Equal(t, 2+1, conf.MessagesReferenced) // 2 + milestone itself
	require.Equal(t, 1, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageD.GetMessageID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 1_500_000)
}

func TestWhiteFlagWithDust(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	// Valid transfer from seed1 (2_779_530_283_277_761) to seed2 (1_000_000)
	messageA, messageAConsumedOutputs, messageASentOutput, messageARemainderOutput := utils.MsgWithValueTx(t,
		te.Milestones[0].GetMilestone().MessageID,
		te.Milestones[1].GetMilestone().MessageID,
		"A",
		seed1Wallet,
		seed2Wallet,
		1_000_000,
		te.PowHandler,
	)
	cachedMessageA := te.StoreMessage(messageA)
	seed1Wallet.BookSpents(messageAConsumedOutputs)
	seed2Wallet.BookOutput(messageASentOutput)
	seed1Wallet.BookOutput(messageARemainderOutput)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Invalid Dust transfer from seed1 (999_999) to seed2
	messageB, _, _, _ := utils.MsgWithValueTx(t,
		cachedMessageA.GetMessage().GetMessageID(),
		te.Milestones[0].GetMilestone().MessageID,
		"B",
		seed1Wallet,
		seed2Wallet,
		999_999,
		te.PowHandler,
	)
	cachedMessageB := te.StoreMessage(messageB)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at message B
	conf := te.IssueAndConfirmMilestoneOnTip(cachedMessageB.GetMessage().GetMessageID(), true)

	require.Equal(t, 2+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageB.GetMessageID(), storage.ConflictInvalidDustAllowance)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_000_000)

	// Dust allowance from seed1 to seed2 with 1_000_000
	messageC, messageCConsumedOutputs, messageCSentOutput, messageCRemainderOutput := utils.MsgWithDustAllowance(t,
		te.Milestones[1].GetMilestone().MessageID,
		cachedMessageB.GetMessage().GetMessageID(),
		"C",
		seed1Wallet,
		seed2Wallet,
		1_000_000,
		te.PowHandler,
	)
	cachedMessageC := te.StoreMessage(messageC)
	seed1Wallet.BookSpents(messageCConsumedOutputs)
	seed2Wallet.BookOutput(messageCSentOutput)
	seed1Wallet.BookOutput(messageCRemainderOutput)
	seed2WalletDustAllowanceOutput := messageCSentOutput

	// Send Dust from seed1 to seed2 with 1
	messageD, messageDConsumedOutputs, messageDSentOutput, messageDRemainderOutput := utils.MsgWithValueTx(t,
		cachedMessageB.GetMessage().GetMessageID(),
		cachedMessageC.GetMessage().GetMessageID(),
		"D",
		seed1Wallet,
		seed2Wallet,
		1,
		te.PowHandler,
	)
	cachedMessageD := te.StoreMessage(messageD)
	seed1Wallet.BookSpents(messageDConsumedOutputs)
	seed2Wallet.BookOutput(messageDSentOutput)
	seed1Wallet.BookOutput(messageDRemainderOutput)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at message D
	conf = te.IssueAndConfirmMilestoneOnTip(cachedMessageD.GetMessage().GetMessageID(), true)

	require.Equal(t, 2+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_281_277_760)
	te.AssertWalletBalance(seed2Wallet, 2_000_001)

	// Spend dust allowance from seed2 (2_000_001) to seed3 (0) (dust allowance)
	messageE, _, _, _ := utils.MsgWithValueTxUsingGivenUTXO(t,
		te.Milestones[3].GetMilestone().MessageID,
		te.Milestones[2].GetMilestone().MessageID,
		"E",
		seed2Wallet,
		seed3Wallet,
		1_000_000,
		te.PowHandler,
		seed2WalletDustAllowanceOutput,
	)
	cachedMessageE := te.StoreMessage(messageE)

	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	// Confirming milestone at message E
	conf = te.IssueAndConfirmMilestoneOnTip(cachedMessageE.GetMessage().GetMessageID(), true)

	require.Equal(t, 1+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageE.GetMessageID(), storage.ConflictInvalidDustAllowance)

	// Verify that the dust allowance is still unspent
	unspent, err := te.UTXO().IsOutputUnspentWithoutLocking(seed2WalletDustAllowanceOutput)
	require.NoError(t, err)
	require.True(t, unspent)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_281_277_760)
	te.AssertWalletBalance(seed2Wallet, 2_000_001)
	te.AssertWalletBalance(seed3Wallet, 0)

	// Spend all outputs, including dust allowance, from seed2 (2_000_001) to seed3 (0)
	messageF, messageFConsumedOutputs, messageFSentOutput, messageFRemainderOutput := utils.MsgWithValueTx(t,
		te.Milestones[3].GetMilestone().MessageID,
		te.Milestones[4].GetMilestone().MessageID,
		"F",
		seed2Wallet,
		seed3Wallet,
		2_000_001,
		te.PowHandler,
	)
	cachedMessageF := te.StoreMessage(messageF)
	seed2Wallet.BookSpents(messageFConsumedOutputs)
	seed3Wallet.BookOutput(messageFSentOutput)
	seed2Wallet.BookOutput(messageFRemainderOutput)

	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	// Confirming milestone at message F
	conf = te.IssueAndConfirmMilestoneOnTip(cachedMessageF.GetMessage().GetMessageID(), true)

	require.Equal(t, 1+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify that the dust allowance spent
	unspent, err = te.UTXO().IsOutputUnspentWithoutLocking(seed2WalletDustAllowanceOutput)
	require.NoError(t, err)
	require.False(t, unspent)

	// Verify balances
	te.AssertWalletBalance(seed2Wallet, 0)
	te.AssertWalletBalance(seed3Wallet, 2_000_001)

}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	genesisWallet := utils.NewHDWallet("Seed1", seed1, 0)
	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 3, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	genesisWallet.BookOutput(te.GenesisOutput)

	// Issue some transactions
	messageA := te.StoreMessage(utils.MsgWithIndexation(t, te.Milestones[0].GetMilestone().MessageID, te.Milestones[1].GetMilestone().MessageID, "A", te.PowHandler))
	messageB := te.StoreMessage(utils.MsgWithIndexation(t, messageA.GetMessage().GetMessageID(), te.Milestones[0].GetMilestone().MessageID, "B", te.PowHandler))
	messageC := te.StoreMessage(utils.MsgWithIndexation(t, te.Milestones[2].GetMilestone().MessageID, te.Milestones[0].GetMilestone().MessageID, "C", te.PowHandler))
	messageD := te.StoreMessage(utils.MsgWithIndexation(t, messageB.GetMessage().GetMessageID(), messageC.GetMessage().GetMessageID(), "D", te.PowHandler))
	messageE := te.StoreMessage(utils.MsgWithIndexation(t, messageB.GetMessage().GetMessageID(), messageA.GetMessage().GetMessageID(), "E", te.PowHandler))

	// Confirming milestone include all msg up to message E. This should only include A, B and E
	conf := te.IssueAndConfirmMilestoneOnTip(messageE.GetMessage().GetMessageID(), true)
	require.Equal(t, 3+1, conf.MessagesReferenced) // A, B, E + 1 for Milestone
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, conf.MessagesExcludedWithoutTransactions) // 1 is for the milestone itself

	// Issue another message
	messageF := te.StoreMessage(utils.MsgWithIndexation(t, messageD.GetMessage().GetMessageID(), messageE.GetMessage().GetMessageID(), "F", te.PowHandler))

	// Confirming milestone at message F. This should confirm D, C and F
	conf = te.IssueAndConfirmMilestoneOnTip(messageF.GetMessage().GetMessageID(), true)

	require.Equal(t, 3+1, conf.MessagesReferenced) // D, C, F + 1 for Milestone
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, conf.MessagesExcludedWithoutTransactions) // 1 is for the milestone itself
}
