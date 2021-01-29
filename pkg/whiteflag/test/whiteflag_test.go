package test

import (
	"encoding/hex"
	"fmt"
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

func TestWhiteFlagSendAllCoins(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, 2_779_530_283_277_761)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	messageA := te.NewMessageBuilder("A").
		Parents(te.Milestones[0].GetMilestone().MessageID, te.Milestones[1].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(2_779_530_283_277_761).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	messageB := te.NewMessageBuilder("B").
		Parents(messageA.StoredMessageID(), te.Milestones[1].GetMilestone().MessageID).
		FromWallet(seed2Wallet).
		ToWallet(seed1Wallet).
		Amount(2_779_530_283_277_761).
		Build().
		Store().
		BookOnWallets()

	// Confirming milestone at message C (message D and E are not included)
	conf := te.IssueAndConfirmMilestoneOnTip(messageB.StoredMessageID(), true)
	require.Equal(t, 2+1, conf.MessagesReferenced) // 2 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_283_277_761)
	te.AssertWalletBalance(seed2Wallet, 0)
}

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
	messageA := te.NewMessageBuilder("A").
		Parents(te.Milestones[0].GetMilestone().MessageID, te.Milestones[1].GetMilestone().MessageID).
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
		Parents(messageA.StoredMessageID(), te.Milestones[0].GetMilestone().MessageID).
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
		Parents(te.Milestones[2].GetMilestone().MessageID, messageB.StoredMessageID()).
		FromWallet(seed3Wallet).
		ToWallet(seed2Wallet).
		Amount(100_000).
		FakeInputs().
		Build().
		Store()

	// Confirming milestone at message C (message D and E are not included)
	conf := te.IssueAndConfirmMilestoneOnTip(messageC.StoredMessageID(), true)
	require.Equal(t, 3+1, conf.MessagesReferenced) // 3 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageC.StoredMessageID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 3_000_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 0)

	// Invalid transfer from seed4 (0) to seed2 (1_500_000) (invalid input)
	messageD := te.NewMessageBuilder("D").
		Parents(messageA.StoredMessageID(), messageC.StoredMessageID()).
		FromWallet(seed4Wallet).
		ToWallet(seed2Wallet).
		Amount(1_500_000).
		FakeInputs().
		Build().
		Store()

	// Valid transfer from seed2 (1_000_000) and seed2 (2_000_000) with remainder seed2 (1_500_000) to seed4 (1_500_000)
	messageE := te.NewMessageBuilder("E").
		Parents(messageB.StoredMessageID(), messageD.StoredMessageID()).
		FromWallet(seed2Wallet).
		ToWallet(seed4Wallet).
		Amount(1_500_000).
		Build().
		Store().
		BookOnWallets()

	seed2Wallet.PrintStatus()
	seed4Wallet.PrintStatus()

	// Confirming milestone at message E
	conf = te.IssueAndConfirmMilestoneOnTip(messageE.StoredMessageID(), true)
	require.Equal(t, 2+1, conf.MessagesReferenced) // 2 + milestone itself
	require.Equal(t, 1, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageD.StoredMessageID(), storage.ConflictInputUTXONotFound)

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
	// Valid transfer from seed1 (1_000_000) to seed2
	messageA := te.NewMessageBuilder("A").
		Parents(te.Milestones[0].GetMilestone().MessageID, te.Milestones[1].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1_000_000).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Invalid Dust transfer from seed1 (999_999) to seed2
	messageB := te.NewMessageBuilder("B").
		Parents(messageA.StoredMessageID(), te.Milestones[0].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(999_999).
		Build().
		Store()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at message B
	conf := te.IssueAndConfirmMilestoneOnTip(messageB.StoredMessageID(), true)

	require.Equal(t, 2+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageB.StoredMessageID(), storage.ConflictInvalidDustAllowance)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_000_000)

	// Dust allowance from seed1 to seed2 with 1_000_000
	messageC := te.NewMessageBuilder("C").
		Parents(te.Milestones[1].GetMilestone().MessageID, messageB.StoredMessageID()).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1_000_000).
		DustAllowance().
		Build().
		Store().
		BookOnWallets()

	// Store the dust allowance output we created, so we can try to spend it
	seed2WalletDustAllowanceOutput := messageC.GeneratedUTXO()

	// Send Dust from seed1 to seed2 with 1
	messageD := te.NewMessageBuilder("D").
		Parents(messageB.StoredMessageID(), messageC.StoredMessageID()).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at message D
	conf = te.IssueAndConfirmMilestoneOnTip(messageD.StoredMessageID(), true)

	require.Equal(t, 2+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_281_277_760)
	te.AssertWalletBalance(seed2Wallet, 2_000_001)

	// Spend dust allowance from seed2 (2_000_001) to seed3 (0) (failure: invalid dust allowance)
	messageE := te.NewMessageBuilder("E").
		Parents(te.Milestones[3].GetMilestone().MessageID, te.Milestones[2].GetMilestone().MessageID).
		FromWallet(seed2Wallet).
		ToWallet(seed3Wallet).
		Amount(1_000_000).
		UsingOutput(seed2WalletDustAllowanceOutput).
		Build().
		Store()

	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	// Confirming milestone at message E
	conf = te.IssueAndConfirmMilestoneOnTip(messageE.StoredMessageID(), true)

	require.Equal(t, 1+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageE.StoredMessageID(), storage.ConflictInvalidDustAllowance)

	// Verify that the dust allowance is still unspent
	unspent, err := te.UTXO().IsOutputUnspentWithoutLocking(seed2WalletDustAllowanceOutput)
	require.NoError(t, err)
	require.True(t, unspent)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_281_277_760)
	te.AssertWalletBalance(seed2Wallet, 2_000_001)
	te.AssertWalletBalance(seed3Wallet, 0)

	// Spend all outputs, including dust allowance, from seed2 (2_000_001) to seed3 (0)
	messageF := te.NewMessageBuilder("F").
		Parents(te.Milestones[3].GetMilestone().MessageID, te.Milestones[4].GetMilestone().MessageID).
		FromWallet(seed2Wallet).
		ToWallet(seed3Wallet).
		Amount(2_000_001).
		Build().
		Store().
		BookOnWallets()

	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	// Confirming milestone at message F
	conf = te.IssueAndConfirmMilestoneOnTip(messageF.StoredMessageID(), true)

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

func TestWhiteFlagDustAllowanceWithLotsOfDust(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	// Valid transfer from seed1 (2_779_530_283_277_761) to seed2 (1_000_000)
	messageA := te.NewMessageBuilder("A").
		Parents(te.Milestones[0].GetMilestone().MessageID, te.Milestones[1].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1_000_000).
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Dust allowance from seed2 to seed2 with 1_000_000
	messageB := te.NewMessageBuilder("B").
		Parents(messageA.StoredMessageID(), te.Milestones[1].GetMilestone().MessageID).
		FromWallet(seed2Wallet).
		ToWallet(seed2Wallet).
		Amount(1_000_000).
		DustAllowance().
		Build().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at message B
	conf := te.IssueAndConfirmMilestoneOnTip(messageB.StoredMessageID(), true)

	require.Equal(t, 2+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_000_000)

	// Generate lots of dust messages
	lastDustMessage := messageA
	var totalDustTxCount int
	var dustTxCount int
	for i := 0; i < 10; i++ {
		// Dust from seed1 to seed2 with 1
		lastDustMessage = te.NewMessageBuilder(fmt.Sprintf("C%d", i)).
			Parents(lastDustMessage.StoredMessageID(), te.Milestones[2].GetMilestone().MessageID).
			FromWallet(seed1Wallet).
			ToWallet(seed2Wallet).
			Amount(1).
			Build().
			Store().
			BookOnWallets()
		dustTxCount++
	}

	require.NotNil(t, lastDustMessage)

	// Confirming milestone at last Dust message
	conf = te.IssueAndConfirmMilestoneOnTip(lastDustMessage.StoredMessageID(), true)

	require.Equal(t, dustTxCount+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, dustTxCount, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	totalDustTxCount += dustTxCount

	require.Equal(t, 10, totalDustTxCount)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761-uint64(totalDustTxCount))
	te.AssertWalletBalance(seed2Wallet, 1_000_000+uint64(totalDustTxCount))

	// Dust from seed1 to seed2 with 1 (failure: dust allowance)
	messageD := te.NewMessageBuilder("D").
		Parents(lastDustMessage.StoredMessageID(), te.Milestones[3].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1).
		Build().
		Store()

	// Confirming milestone at message D
	conf = te.IssueAndConfirmMilestoneOnTip(messageD.StoredMessageID(), true)

	require.Equal(t, 1+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageD.StoredMessageID(), storage.ConflictInvalidDustAllowance)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761-uint64(totalDustTxCount))
	te.AssertWalletBalance(seed2Wallet, 1_000_000+uint64(totalDustTxCount))

	// More Dust allowance from seed1 to seed2 with 1_000_000
	messageE := te.NewMessageBuilder("E").
		Parents(messageD.StoredMessageID(), te.Milestones[3].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(10_000_000).
		DustAllowance().
		Build().
		Store().
		BookOnWallets()

	// Generate lots of dust messages from seed1 to seed2
	lastDustMessage = messageE
	dustTxCount = 0
	for i := 0; i < 100-totalDustTxCount; i++ {
		// Dust from seed1 to seed2 with 1
		lastDustMessage = te.NewMessageBuilder(fmt.Sprintf("F%d", i)).
			Parents(lastDustMessage.StoredMessageID(), te.Milestones[3].GetMilestone().MessageID).
			FromWallet(seed1Wallet).
			ToWallet(seed2Wallet).
			Amount(1).
			Build().
			Store().
			BookOnWallets()
		dustTxCount++
	}

	totalDustTxCount += dustTxCount

	require.Equal(t, 100, totalDustTxCount)

	// Confirming milestone at message F
	conf = te.IssueAndConfirmMilestoneOnTip(lastDustMessage.StoredMessageID(), true)

	require.Equal(t, 1+dustTxCount+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 1+dustTxCount, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761-uint64(totalDustTxCount)-10_000_000)
	te.AssertWalletBalance(seed2Wallet, 1_000_000+uint64(totalDustTxCount)+10_000_000)

	// Dust from seed1 to seed2 with 1 (failure: dust allowance)
	messageG := te.NewMessageBuilder("G").
		Parents(lastDustMessage.StoredMessageID(), te.Milestones[4].GetMilestone().MessageID).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1).
		Build().
		Store()

	// Confirming milestone at message G
	conf = te.IssueAndConfirmMilestoneOnTip(messageG.StoredMessageID(), true)

	require.Equal(t, 1+1, conf.MessagesReferenced) // 1 + milestone itself
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify the messages have the expected conflict reason
	te.AssertMessageConflictReason(messageG.StoredMessageID(), storage.ConflictInvalidDustAllowance)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_282_277_761-uint64(totalDustTxCount)-10_000_000)
	te.AssertWalletBalance(seed2Wallet, 1_000_000+uint64(totalDustTxCount)+10_000_000)
}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	genesisWallet := utils.NewHDWallet("Seed1", seed1, 0)
	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 3, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	//Add token supply to our local HDWallet
	genesisWallet.BookOutput(te.GenesisOutput)

	// Issue some transactions
	messageA := te.NewMessageBuilder("A").Parents(te.Milestones[0].GetMilestone().MessageID, te.Milestones[1].GetMilestone().MessageID).BuildIndexation().Store()
	messageB := te.NewMessageBuilder("B").Parents(messageA.StoredMessageID(), te.Milestones[0].GetMilestone().MessageID).BuildIndexation().Store()
	messageC := te.NewMessageBuilder("C").Parents(te.Milestones[2].GetMilestone().MessageID, te.Milestones[0].GetMilestone().MessageID).BuildIndexation().Store()
	messageD := te.NewMessageBuilder("D").Parents(messageB.StoredMessageID(), messageC.StoredMessageID()).BuildIndexation().Store()
	messageE := te.NewMessageBuilder("E").Parents(messageB.StoredMessageID(), messageA.StoredMessageID()).BuildIndexation().Store()

	// Confirming milestone include all msg up to message E. This should only include A, B and E
	conf := te.IssueAndConfirmMilestoneOnTip(messageE.StoredMessageID(), true)
	require.Equal(t, 3+1, conf.MessagesReferenced) // A, B, E + 1 for Milestone
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, conf.MessagesExcludedWithoutTransactions) // 1 is for the milestone itself

	// Issue another message
	messageF := te.NewMessageBuilder("F").Parents(messageD.StoredMessageID(), messageE.StoredMessageID()).BuildIndexation().Store()

	// Confirming milestone at message F. This should confirm D, C and F
	conf = te.IssueAndConfirmMilestoneOnTip(messageF.StoredMessageID(), true)

	require.Equal(t, 3+1, conf.MessagesReferenced) // D, C, F + 1 for Milestone
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, conf.MessagesExcludedWithoutTransactions) // 1 is for the milestone itself
}
