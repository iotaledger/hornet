package test

import (
	"encoding/hex"
	"testing"

	_ "golang.org/x/crypto/blake2b"

	iotago "github.com/iotaledger/iota.go"
	"github.com/stretchr/testify/require"

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

	genesisAddress := utils.GenerateHDWalletAddress(t, seed1, 0)
	genesisUTXO := &iotago.UTXOInput{}

	// Fill up the balances
	balances := make(map[string]uint64)
	balances[string(genesisAddress.String())] = 1000

	te := testsuite.SetupTestEnvironment(t, balances, 3, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	// Issue some transactions
	// Valid transfer from seed1[0] (1000) with remainder seed1[1] (900) to seed2[0]_A (100)
	messageA, utxoSeedOneAccountOne, utxoSeedTwoAccountZeroA := utils.MsgWithValueTx(t, te.Milestones[0].GetMessage().GetMessageID(), te.Milestones[1].GetMessage().GetMessageID(),
		"A", []*iotago.UTXOInput{genesisUTXO}, seed1, []uint64{0}, []uint64{1000}, 1, seed2, 0, 100)
	chachedMessageA := te.StoreMessage(messageA)

	// Valid transfer from seed1[1] (900) with remainder seed1[2] (700) to seed2[0]_B (200)
	messageB, utxoSeedOneAccountTwo, utxoSeedTwoAccountZeroB := utils.MsgWithValueTx(t, chachedMessageA.GetMessage().GetMessageID(), te.Milestones[0].GetMessage().GetMessageID(),
		"B", []*iotago.UTXOInput{utxoSeedOneAccountOne}, seed1, []uint64{1}, []uint64{900}, 2, seed2, 0, 200)
	chachedMessageB := te.StoreMessage(messageB)

	// Invalid transfer from seed3[0] (0) to seed2[0] (10) (insufficient funds)
	messageC, _, _ := utils.MsgWithValueTx(t, te.Milestones[2].GetMessage().GetMessageID(), chachedMessageB.GetMessage().GetMessageID(),
		"C", []*iotago.UTXOInput{utxoSeedOneAccountTwo}, seed3, []uint64{0}, []uint64{99999}, 0, seed2, 0, 10)
	chachedMessageC := te.StoreMessage(messageC)

	// Invalid transfer from seed4[1] (0) to seed2[0] (150) (insufficient funds)
	messageD, _, _ := utils.MsgWithValueTx(t, chachedMessageA.GetMessage().GetMessageID(), chachedMessageC.GetMessage().GetMessageID(),
		"D", []*iotago.UTXOInput{utxoSeedTwoAccountZeroB}, seed4, []uint64{1}, []uint64{99999}, 0, seed2, 0, 150)
	chachedMessageD := te.StoreMessage(messageD)

	// Valid transfer from seed2[0]_A (100) and seed2[0]_B (200) with remainder seed2[1] (150) to seed4[0] (150)
	messageE, _, _ := utils.MsgWithValueTx(t, chachedMessageB.GetMessage().GetMessageID(), chachedMessageD.GetMessage().GetMessageID(),
		"E", []*iotago.UTXOInput{utxoSeedTwoAccountZeroA, utxoSeedTwoAccountZeroB}, seed2, []uint64{0, 0}, []uint64{100, 200}, 1, seed4, 0, 150)
	chachedMessageE := te.StoreMessage(messageE)

	// Confirming milestone at message C (message D and E are not included)
	conf := te.IssueAndConfirmMilestoneOnTip(chachedMessageC.GetMessage().GetMessageID(), true)

	require.Equal(t, 3+1, conf.MessagesReferenced) // 3 + milestone itself
	require.Equal(t, 2, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances (seed, index, balance)
	te.AssertAddressBalance(seed1, 0, 0)
	te.AssertAddressBalance(seed1, 1, 0)
	te.AssertAddressBalance(seed1, 2, 700)
	te.AssertAddressBalance(seed2, 0, 300)
	te.AssertAddressBalance(seed2, 1, 0)
	te.AssertAddressBalance(seed3, 0, 0)
	te.AssertAddressBalance(seed3, 1, 0)
	te.AssertAddressBalance(seed4, 0, 0)
	te.AssertAddressBalance(seed4, 1, 0)

	// Confirming milestone at message E
	conf = te.IssueAndConfirmMilestoneOnTip(chachedMessageE.GetMessage().GetMessageID(), true)
	require.Equal(t, 2+1, conf.MessagesReferenced) // 2 + milestone itself
	require.Equal(t, 1, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 1, conf.MessagesExcludedWithoutTransactions) // the milestone

	// Verify balances (seed, index, balance)
	te.AssertAddressBalance(seed1, 0, 0)
	te.AssertAddressBalance(seed1, 1, 0)
	te.AssertAddressBalance(seed1, 2, 700)
	te.AssertAddressBalance(seed2, 0, 0)
	te.AssertAddressBalance(seed2, 1, 150)
	te.AssertAddressBalance(seed3, 0, 0)
	te.AssertAddressBalance(seed3, 1, 0)
	te.AssertAddressBalance(seed4, 0, 150)
	te.AssertAddressBalance(seed4, 1, 0)
}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	// Fill up the balances
	balances := make(map[string]uint64)
	te := testsuite.SetupTestEnvironment(t, balances, 3, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	// Issue some transactions
	messageA := te.StoreMessage(utils.MsgWithIndexation(t, te.Milestones[0].GetMessage().GetMessageID(), te.Milestones[1].GetMessage().GetMessageID(), "A"))
	messageB := te.StoreMessage(utils.MsgWithIndexation(t, messageA.GetMessage().GetMessageID(), te.Milestones[0].GetMessage().GetMessageID(), "B"))
	messageC := te.StoreMessage(utils.MsgWithIndexation(t, te.Milestones[2].GetMessage().GetMessageID(), te.Milestones[0].GetMessage().GetMessageID(), "C"))
	messageD := te.StoreMessage(utils.MsgWithIndexation(t, messageB.GetMessage().GetMessageID(), messageC.GetMessage().GetMessageID(), "D"))
	messageE := te.StoreMessage(utils.MsgWithIndexation(t, messageB.GetMessage().GetMessageID(), messageA.GetMessage().GetMessageID(), "E"))

	// Confirming milestone include all msg up to message E. This should only include A, B and E
	conf := te.IssueAndConfirmMilestoneOnTip(messageE.GetMessage().GetMessageID(), true)
	require.Equal(t, 3+1, conf.MessagesReferenced) // A, B, E + 3 for Milestone
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, conf.MessagesExcludedWithoutTransactions) // 3 are for the milestone itself

	// Issue another message
	messageF := te.StoreMessage(utils.MsgWithIndexation(t, messageD.GetMessage().GetMessageID(), messageE.GetMessage().GetMessageID(), "F"))

	// Confirming milestone at message F. This should confirm D, C and F
	conf = te.IssueAndConfirmMilestoneOnTip(messageF.GetMessage().GetMessageID(), true)

	require.Equal(t, 3+1, conf.MessagesReferenced) // D, C, F + 3 for Milestone
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, conf.MessagesExcludedWithoutTransactions) // 3 are for the milestone itself
}
