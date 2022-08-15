//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/testsuite"
	"github.com/iotaledger/hornet/v2/pkg/testsuite/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	ShowConfirmationGraphs = false
	ProtocolVersion        = 2
	MinPoWScore            = 1
	BelowMaxDepth          = 15
)

var (
	seed1, _ = hex.DecodeString("96d9ff7a79e4b0a5f3e5848ae7867064402da92a62eabb4ebbe463f12d1f3b1aace1775488f51cb1e3a80732a03ef60b111d6833ab605aa9f8faebeb33bbe3d9")
	seed2, _ = hex.DecodeString("b15209ddc93cbdb600137ea6a8f88cdd7c5d480d5815c9352a0fb5c4e4b86f7151dcb44c2ba635657a2df5a8fd48cb9bab674a9eceea527dbbb254ef8c9f9cd7")
	seed3, _ = hex.DecodeString("d5353ceeed380ab89a0f6abe4630c2091acc82617c0edd4ff10bd60bba89e2ed30805ef095b989c2bf208a474f8748d11d954aade374380422d4d812b6f1da90")
	seed4, _ = hex.DecodeString("bd6fe09d8a309ca309c5db7b63513240490109cd0ac6b123551e9da0d5c8916c4a5a4f817e4b4e9df89885ce1af0986da9f1e56b65153c2af1e87ab3b11dabb4")
)

func TestWhiteFlagSendAllCoins(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed2Wallet).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // 1 + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 0)
	te.AssertWalletBalance(seed2Wallet, te.ProtocolParameters().TokenSupply)

	// Issue some transactions
	blockB := te.NewBlockBuilder("B").
		Parents(append(te.LastMilestoneParents(), blockA.StoredBlockID())).
		FromWallet(seed2Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed1Wallet).
		Store().
		BookOnWallets()

	// Confirming milestone at block C (block D and E are not included)
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // 1 + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)
	te.AssertWalletBalance(seed2Wallet, 0)
}

func TestWhiteFlagWithMultipleConflicting(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)
	seed4Wallet := utils.NewHDWallet("Seed4", seed4, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	// Valid transfer from seed1 (iotago.TokenSupply) with remainder seed1 (2_779_530_282_277_761) to seed2 (1_000_000)
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(1_000_000).
		BuildTransactionToWallet(seed2Wallet).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Valid transfer from seed1 (2_779_530_282_277_761) with remainder seed1 (2_779_530_280_277_761) to seed2 (2_000_000)
	blockB := te.NewBlockBuilder("B").
		Parents(append(te.LastMilestoneParents(), blockA.StoredBlockID())).
		FromWallet(seed1Wallet).
		Amount(2_000_000).
		BuildTransactionToWallet(seed2Wallet).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Invalid transfer from seed3 (0) to seed2 (1_000_000) (invalid input)
	blockC := te.NewBlockBuilder("C").
		Parents(append(te.LastMilestoneParents(), blockB.StoredBlockID())).
		FromWallet(seed3Wallet).
		Amount(1_000_000).
		FakeInputs().
		BuildTransactionToWallet(seed2Wallet).
		Store()

	// Confirming milestone at block C (block D and E are not included)
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockC.StoredBlockID()}, true)
	require.Equal(t, 3+1, confStats.BlocksReferenced) // 3 + previous milestone
	require.Equal(t, 2, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockC.StoredBlockID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 3_000_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 0)

	// Invalid transfer from seed4 (0) to seed2 (1_500_000) (invalid input)
	blockD := te.NewBlockBuilder("D").
		Parents(iotago.BlockIDs{blockA.StoredBlockID(), blockC.StoredBlockID()}).
		FromWallet(seed4Wallet).
		Amount(1_500_000).
		FakeInputs().
		BuildTransactionToWallet(seed2Wallet).
		Store()

	// Valid transfer from seed2 (1_000_000) and seed2 (2_000_000) with remainder seed2 (1_500_000) to seed4 (1_500_000)
	blockE := te.NewBlockBuilder("E").
		Parents(iotago.BlockIDs{blockB.StoredBlockID(), blockD.StoredBlockID()}).
		FromWallet(seed2Wallet).
		Amount(1_500_000).
		BuildTransactionToWallet(seed4Wallet).
		Store().
		BookOnWallets()

	seed4WalletOutput := blockE.GeneratedUTXO()

	seed2Wallet.PrintStatus()
	seed4Wallet.PrintStatus()

	// Confirming milestone at block E
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockE.StoredBlockID()}, true)
	require.Equal(t, 2+1, confStats.BlocksReferenced) // 2 + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockD.StoredBlockID(), storage.ConflictInputUTXONotFound)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 1_500_000)

	// Invalid transfer from seed3 (0) to seed2 (1_000_000) (already spent (genesis))
	blockF := te.NewBlockBuilder("F").
		Parents(append(te.LastMilestoneParents(), blockE.StoredBlockID())).
		FromWallet(seed3Wallet).
		Amount(1_000_000).
		UsingOutput(te.GenesisOutput).
		BuildTransactionToWallet(seed2Wallet).
		Store()

	// Confirming milestone at block F
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockF.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // 1 + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockF.StoredBlockID(), storage.ConflictInputUTXOAlreadySpent)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 0)
	te.AssertWalletBalance(seed4Wallet, 1_500_000)

	// Valid transfer from seed4 to seed3 (1_500_000)
	blockG := te.NewBlockBuilder("G").
		Parents(append(te.LastMilestoneParents(), blockF.StoredBlockID())).
		FromWallet(seed4Wallet).
		Amount(1_500_000).
		UsingOutput(seed4WalletOutput).
		BuildTransactionToWallet(seed3Wallet).
		Store().
		BookOnWallets()

	// Valid transfer from seed4 to seed2 (1_500_000) (double spend -> already spent)
	blockH := te.NewBlockBuilder("H").
		Parents(iotago.BlockIDs{blockG.StoredBlockID()}).
		FromWallet(seed4Wallet).
		Amount(1_500_000).
		UsingOutput(seed4WalletOutput).
		BuildTransactionToWallet(seed2Wallet).
		Store()

	// Confirming milestone at block H
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockH.StoredBlockID()}, true)
	require.Equal(t, 2+1, confStats.BlocksReferenced) // 1 + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockH.StoredBlockID(), storage.ConflictInputUTXOAlreadySpentInThisMilestone)

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 2_779_530_280_277_761)
	te.AssertWalletBalance(seed2Wallet, 1_500_000)
	te.AssertWalletBalance(seed3Wallet, 1_500_000)
	te.AssertWalletBalance(seed4Wallet, 0)
}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	genesisWallet := utils.NewHDWallet("Seed1", seed1, 0)
	genesisAddress := genesisWallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 3, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	genesisWallet.BookOutput(te.GenesisOutput)

	// Issue some transactions
	blockA := te.NewBlockBuilder("A").Parents(te.LastMilestoneParents()).BuildTaggedData().Store()
	blockB := te.NewBlockBuilder("B").Parents(append(te.LastMilestoneParents(), blockA.StoredBlockID())).BuildTaggedData().Store()
	blockC := te.NewBlockBuilder("C").Parents(te.LastMilestoneParents()).BuildTaggedData().Store()
	blockD := te.NewBlockBuilder("D").Parents(iotago.BlockIDs{blockB.StoredBlockID(), blockC.StoredBlockID()}).BuildTaggedData().Store()
	blockE := te.NewBlockBuilder("E").Parents(iotago.BlockIDs{blockB.StoredBlockID(), blockA.StoredBlockID()}).BuildTaggedData().Store()

	// Confirming milestone include all blocks up to block E. This should only include A, B and E
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockE.StoredBlockID()}, true)
	require.Equal(t, 3+1, confStats.BlocksReferenced) // A, B, E + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, confStats.BlocksExcludedWithoutTransactions) // 1 is for previous milestone

	// Issue another block
	blockF := te.NewBlockBuilder("F").Parents(iotago.BlockIDs{blockD.StoredBlockID(), blockE.StoredBlockID()}).BuildTaggedData().Store()

	// Confirming milestone at block F. This should confirm D, C and F
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockF.StoredBlockID()}, true)

	require.Equal(t, 3+1, confStats.BlocksReferenced) // D, C, F + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 3+1, confStats.BlocksExcludedWithoutTransactions) // 1 is for previous milestone
}

func TestWhiteFlagLastMilestoneNotInPastCone(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed2Wallet).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 0)
	te.AssertWalletBalance(seed2Wallet, te.ProtocolParameters().TokenSupply)

	// Issue some transactions
	blockB := te.NewBlockBuilder("B").
		Parents(append(te.LastMilestoneParents(), blockA.StoredBlockID())).
		FromWallet(seed2Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed1Wallet).
		Store().
		BookOnWallets()

	// Issue milestone 5 that does not include the milestone 4 in the past
	_, _, err := te.IssueMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, false)
	require.Error(t, err)
}

func TestWhiteFlagConfirmWithReattachedMilestone(t *testing.T) {

	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Issue some transactions
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed2Wallet).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify balances
	te.AssertWalletBalance(seed1Wallet, 0)
	te.AssertWalletBalance(seed2Wallet, te.ProtocolParameters().TokenSupply)

	// Issue some transactions
	blockB := te.NewBlockBuilder("B").
		Parents(append(te.LastMilestoneParents(), blockA.StoredBlockID())).
		FromWallet(seed2Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed1Wallet).
		Store().
		BookOnWallets()

	// Issue milestone 5
	milestone5, blockIDMilestone5, err := te.IssueMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.NoError(t, err)

	_, confStats = te.ConfirmMilestone(milestone5, false)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // B + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Valid reattachment with same parents and different nonce
	milestone5Reattachment := te.ReattachBlock(blockIDMilestone5)

	// Invalid reattachment with different parents
	invalidMilestone5Reattachment := te.ReattachBlock(blockIDMilestone5, blockB.StoredBlockID(), iotago.EmptyBlockID())

	// Issue a transaction referencing the milestone5 reattached block specifically
	blockC := te.NewBlockBuilder("C").
		Parents(iotago.BlockIDs{blockB.StoredBlockID(), milestone5Reattachment}).
		FromWallet(seed1Wallet).
		Amount(te.ProtocolParameters().TokenSupply).
		BuildTransactionToWallet(seed2Wallet).
		Store().
		BookOnWallets()

	// Issue milestone 6 that confirms a block that is attached to the reattached milestone 5 and the reattached milestone 5 (leaving 5 unconfirmed)
	milestone6, _, err := te.IssueMilestoneOnTips(iotago.BlockIDs{blockC.StoredBlockID(), milestone5Reattachment}, false)
	require.NoError(t, err)
	_, confStats = te.ConfirmMilestone(milestone6, false)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // 1 +  reattachment
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // reattachment

	milestone5Metadata := te.Storage().CachedBlockMetadataOrNil(blockIDMilestone5)
	require.NotNil(t, milestone5Metadata)
	defer milestone5Metadata.Release(true)
	require.False(t, milestone5Metadata.Metadata().IsReferenced())
	require.True(t, milestone5Metadata.Metadata().IsMilestone())

	reattachmentMetadata := te.Storage().CachedBlockMetadataOrNil(milestone5Reattachment)
	require.NotNil(t, reattachmentMetadata)
	defer reattachmentMetadata.Release(true)
	require.True(t, reattachmentMetadata.Metadata().IsReferenced())
	require.True(t, reattachmentMetadata.Metadata().IsMilestone())

	invalidMilestone5Metadata := te.Storage().CachedBlockMetadataOrNil(invalidMilestone5Reattachment)
	require.NotNil(t, invalidMilestone5Metadata)
	defer invalidMilestone5Metadata.Release(true)
	require.False(t, invalidMilestone5Metadata.Metadata().IsReferenced())
	require.False(t, invalidMilestone5Metadata.Metadata().IsMilestone())
}

func TestWhiteFlagAliasOutput(t *testing.T) {
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()

	// Create Alias
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		BuildAlias().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 0)

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 1)
	aliasOutput := te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, aliasOutput.Output().(*iotago.AliasOutput).StateIndex, uint32(0))

	// Valid State Transition
	newAliasOutput := aliasOutput.Output().(*iotago.AliasOutput).Clone().(*iotago.AliasOutput)
	newAliasOutput.AliasID = iotago.AliasIDFromOutputID(aliasOutput.OutputID())
	newAliasOutput.StateIndex++

	blockB := te.NewBlockBuilder("B").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(aliasOutput.Deposit()).
		UsingOutput(aliasOutput).
		BuildTransactionSendingOutputsAndCalculateRemainder(newAliasOutput).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()

	// Confirming milestone at block B
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // B + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 1)
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, aliasOutput.Output().(*iotago.AliasOutput).StateIndex, uint32(1))

	// Invalid State Transition
	newAliasOutput = aliasOutput.Output().(*iotago.AliasOutput).Clone().(*iotago.AliasOutput)
	newAliasOutput.StateIndex++
	newAliasOutput.StateIndex++

	blockC := te.NewBlockBuilder("C").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(aliasOutput.Deposit()).
		UsingOutput(aliasOutput).
		BuildTransactionSendingOutputsAndCalculateRemainder(newAliasOutput).
		Store()

	seed1Wallet.PrintStatus()

	// Confirming milestone at block C
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockC.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // C + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockC.StoredBlockID(), storage.ConflictInvalidChainStateTransition)

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 1)
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, aliasOutput.Output().(*iotago.AliasOutput).StateIndex, uint32(1))

	// Valid Governance Transition
	newAliasOutput = aliasOutput.Output().(*iotago.AliasOutput).Clone().(*iotago.AliasOutput)
	newAliasOutput.UnlockConditionSet().StateControllerAddress().Address = seed2Wallet.Address()

	blockD := te.NewBlockBuilder("D").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(aliasOutput.Deposit()).
		UsingOutput(aliasOutput).
		BuildTransactionSendingOutputsAndCalculateRemainder(newAliasOutput).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// Confirming milestone at block D
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockD.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // D + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 1)
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.True(t, aliasOutput.Output().UnlockConditionSet().StateControllerAddress().Address.Equal(seed2Wallet.Address()))

	// Invalid Governance Transition
	newAliasOutput = aliasOutput.Output().(*iotago.AliasOutput).Clone().(*iotago.AliasOutput)
	newAliasOutput.UnlockConditionSet().StateControllerAddress().Address = seed3Wallet.Address()
	newAliasOutput.StateIndex++

	blockE := te.NewBlockBuilder("E").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(aliasOutput.Deposit()).
		UsingOutput(aliasOutput).
		BuildTransactionSendingOutputsAndCalculateRemainder(newAliasOutput).
		Store()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	// Confirming milestone at block E
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockE.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // E + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Burn Alias
	blockF := te.NewBlockBuilder("F").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(aliasOutput.Deposit()).
		UsingOutput(aliasOutput).
		BuildTransactionToWallet(seed1Wallet).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	// Confirming milestone at block C
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockF.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // F + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 0)
}

func TestWhiteFlagFoundryOutput(t *testing.T) {
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()

	// --- Create Alias ---

	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(1_000_000_000).
		BuildAlias().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()

	require.Equal(t, len(te.UnspentAliasOutputsInLedger()), 0)

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput := te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(0), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	// --- Create Foundry ---

	blockB := te.NewBlockBuilder("B").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		BuildFoundryOnAlias(aliasOutput).
		Store().
		BookOnWallets()

	require.Equal(t, 0, len(te.UnspentFoundryOutputsInLedger()))

	// Confirming milestone at block B
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // B + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(1), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	require.Equal(t, 1, len(te.UnspentFoundryOutputsInLedger()))
	foundryOutput := te.UnspentFoundryOutputsInLedger()[0]
	require.Equal(t, uint32(1), foundryOutput.Output().(*iotago.FoundryOutput).SerialNumber)
	te.AssertFoundryTokenScheme(foundryOutput, 0, 0, 1000)

	// --- Mint Tokens ---

	newAlias := aliasOutput.Output().Clone().(*iotago.AliasOutput)
	newFoundry := foundryOutput.Output().Clone().(*iotago.FoundryOutput)

	// Mint tokens in foundry
	newFoundry.TokenScheme.(*iotago.SimpleTokenScheme).MintedTokens = big.NewInt(200)

	// Send the minted tokens to a wallet
	newBasicOutput := &iotago.BasicOutput{
		Amount: 0,
		NativeTokens: iotago.NativeTokens{
			&iotago.NativeToken{
				ID:     newFoundry.MustNativeTokenID(),
				Amount: big.NewInt(200),
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: seed2Wallet.Address()},
		},
		Features: nil,
	}

	// Pay rent for new output holding the minted native tokens
	newBasicOutput.Amount = te.ProtocolParameters().RentStructure.MinRent(newBasicOutput)
	newAlias.Amount -= newBasicOutput.Amount
	newAlias.StateIndex++

	blockC := te.NewBlockBuilder("C").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput, foundryOutput}, iotago.Outputs{newAlias, newFoundry, newBasicOutput}, []*utils.HDWallet{seed1Wallet}).
		Store().
		BookOnWallets()

	// Confirming milestone at block C
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockC.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // C + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	require.Equal(t, 1, len(seed2Wallet.Outputs()))
	require.Equal(t, big.NewInt(200), seed2Wallet.Outputs()[0].Output().NativeTokenList().MustSet()[newFoundry.MustNativeTokenID()].Amount)

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(2), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	require.Equal(t, 1, len(te.UnspentFoundryOutputsInLedger()))
	foundryOutput = te.UnspentFoundryOutputsInLedger()[0]
	require.Equal(t, uint32(1), foundryOutput.Output().(*iotago.FoundryOutput).SerialNumber)
	te.AssertFoundryTokenScheme(foundryOutput, 200, 0, 1000)

	// --- Mint Tokens (invalid amount) ---

	newAlias = aliasOutput.Output().Clone().(*iotago.AliasOutput)
	newFoundry = foundryOutput.Output().Clone().(*iotago.FoundryOutput)

	// Mint another 100 tokens in foundry
	newFoundry.TokenScheme.(*iotago.SimpleTokenScheme).MintedTokens = big.NewInt(300)

	// Send too many minted tokens to the wallet
	newBasicOutput = &iotago.BasicOutput{
		Amount: 0,
		NativeTokens: iotago.NativeTokens{
			&iotago.NativeToken{
				ID:     newFoundry.MustNativeTokenID(),
				Amount: big.NewInt(400),
			},
		},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: seed2Wallet.Address()},
		},
		Features: nil,
	}

	// Pay rent for new output holding the minted native tokens
	newBasicOutput.Amount = te.ProtocolParameters().RentStructure.MinRent(newBasicOutput)
	newAlias.Amount -= newBasicOutput.Amount
	newAlias.StateIndex++

	blockD := te.NewBlockBuilder("D").
		Parents(te.LastMilestoneParents()).
		ToWallet(seed2Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput, foundryOutput}, iotago.Outputs{newAlias, newFoundry, newBasicOutput}, []*utils.HDWallet{seed1Wallet}).
		Store()

	// Confirming milestone at block D
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockD.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // D + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockD.StoredBlockID(), storage.ConflictInvalidChainStateTransition)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	require.Equal(t, 1, len(seed2Wallet.Outputs()))
	require.Equal(t, big.NewInt(200), seed2Wallet.Outputs()[0].Output().NativeTokenList().MustSet()[newFoundry.MustNativeTokenID()].Amount)

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(2), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	require.Equal(t, 1, len(te.UnspentFoundryOutputsInLedger()))
	foundryOutput = te.UnspentFoundryOutputsInLedger()[0]
	require.Equal(t, uint32(1), foundryOutput.Output().(*iotago.FoundryOutput).SerialNumber)
	te.AssertFoundryTokenScheme(foundryOutput, 200, 0, 1000)

	// --- Melt 100 tokens ---

	basicOutput := seed2Wallet.Outputs()[0]

	newAlias = aliasOutput.Output().Clone().(*iotago.AliasOutput)
	newFoundry = foundryOutput.Output().Clone().(*iotago.FoundryOutput)

	// Melt 100 tokens in foundry
	newFoundry.TokenScheme.(*iotago.SimpleTokenScheme).MeltedTokens = big.NewInt(100)
	newAlias.StateIndex++

	// Burn the tokens inside the basic output
	newBasicOutput = basicOutput.Output().Clone().(*iotago.BasicOutput)
	newBasicOutput.NativeTokens = iotago.NativeTokens{
		&iotago.NativeToken{
			ID:     newFoundry.MustNativeTokenID(),
			Amount: big.NewInt(100),
		},
	}

	blockE := te.NewBlockBuilder("E").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput, foundryOutput, basicOutput}, iotago.Outputs{newAlias, newFoundry, newBasicOutput}, []*utils.HDWallet{seed1Wallet, seed2Wallet}).
		Store().
		BookOnWallets()

	// Confirming milestone at block E
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockE.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // E + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(seed2Wallet.Outputs()))
	require.Equal(t, big.NewInt(100), seed2Wallet.Outputs()[0].Output().NativeTokenList().MustSet()[newFoundry.MustNativeTokenID()].Amount)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(3), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	require.Equal(t, 1, len(te.UnspentFoundryOutputsInLedger()))
	foundryOutput = te.UnspentFoundryOutputsInLedger()[0]
	require.Equal(t, uint32(1), foundryOutput.Output().(*iotago.FoundryOutput).SerialNumber)
	te.AssertFoundryTokenScheme(foundryOutput, 200, 100, 1000)

	// --- Melt 100 tokens but don't burn enough ---

	basicOutput = seed2Wallet.Outputs()[0]

	newAlias = aliasOutput.Output().Clone().(*iotago.AliasOutput)
	newFoundry = foundryOutput.Output().Clone().(*iotago.FoundryOutput)

	// Melt 100 tokens in foundry
	newFoundry.TokenScheme.(*iotago.SimpleTokenScheme).MeltedTokens = big.NewInt(200)
	newAlias.StateIndex++

	// Burn the tokens inside the basic output
	newBasicOutput = basicOutput.Output().Clone().(*iotago.BasicOutput)
	newBasicOutput.NativeTokens = iotago.NativeTokens{
		&iotago.NativeToken{
			ID:     newFoundry.MustNativeTokenID(),
			Amount: big.NewInt(50),
		},
	}

	blockF := te.NewBlockBuilder("F").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput, foundryOutput, basicOutput}, iotago.Outputs{newAlias, newFoundry, newBasicOutput}, []*utils.HDWallet{seed1Wallet, seed2Wallet}).
		Store()

	// Confirming milestone at block F
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockF.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // F + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockF.StoredBlockID(), storage.ConflictInvalidChainStateTransition)

	require.Equal(t, 1, len(seed2Wallet.Outputs()))
	require.Equal(t, big.NewInt(100), seed2Wallet.Outputs()[0].Output().NativeTokenList().MustSet()[newFoundry.MustNativeTokenID()].Amount)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput = te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(3), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	require.Equal(t, 1, len(te.UnspentFoundryOutputsInLedger()))
	foundryOutput = te.UnspentFoundryOutputsInLedger()[0]
	require.Equal(t, uint32(1), foundryOutput.Output().(*iotago.FoundryOutput).SerialNumber)
	te.AssertFoundryTokenScheme(foundryOutput, 200, 100, 1000)

	// --- Burn tokens ---

	basicOutput = seed2Wallet.Outputs()[0]

	// Burn the tokens inside the basic output
	newBasicOutput = basicOutput.Output().Clone().(*iotago.BasicOutput)
	newBasicOutput.NativeTokens = nil

	blockG := te.NewBlockBuilder("G").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed2Wallet).
		ToWallet(seed2Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{basicOutput}, iotago.Outputs{newBasicOutput}, []*utils.HDWallet{seed2Wallet}).
		Store().
		BookOnWallets()

	// Confirming milestone at block F
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockG.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // G + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(seed2Wallet.Outputs()))
	require.Equal(t, 0, len(seed2Wallet.Outputs()[0].Output().NativeTokenList()))

	// --- Melt tokens without burning (invalid) ---

	newAlias = aliasOutput.Output().Clone().(*iotago.AliasOutput)
	newFoundry = foundryOutput.Output().Clone().(*iotago.FoundryOutput)

	// Melt 100 tokens in foundry
	newFoundry.TokenScheme.(*iotago.SimpleTokenScheme).MeltedTokens = newFoundry.TokenScheme.(*iotago.SimpleTokenScheme).MintedTokens
	newAlias.StateIndex++

	blockH := te.NewBlockBuilder("H").
		Parents(te.LastMilestoneParents()).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput, foundryOutput}, iotago.Outputs{newAlias, newFoundry}, []*utils.HDWallet{seed1Wallet}).
		Store()

	// Confirming milestone at block H
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockH.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // H + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockH.StoredBlockID(), storage.ConflictInvalidChainStateTransition)

}

func TestWhiteFlagFoundryOutputInvalidSerialNumber(t *testing.T) {
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()

	// Create Alias
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(1_000_000_000).
		BuildAlias().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()

	require.Equal(t, 0, len(te.UnspentAliasOutputsInLedger()))

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput := te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(0), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	newAlias := aliasOutput.Output().Clone().(*iotago.AliasOutput)
	if newAlias.AliasID.Empty() {
		newAlias.AliasID = iotago.AliasIDFromOutputID(aliasOutput.OutputID())
	}

	newAlias.StateIndex++
	newAlias.FoundryCounter++

	foundry := &iotago.FoundryOutput{
		Amount:       0,
		NativeTokens: nil,
		SerialNumber: 1337,
		TokenScheme: &iotago.SimpleTokenScheme{
			MintedTokens:  big.NewInt(0),
			MeltedTokens:  big.NewInt(0),
			MaximumSupply: big.NewInt(1000),
		},
		Conditions: iotago.UnlockConditions{
			&iotago.ImmutableAliasUnlockCondition{Address: newAlias.AliasID.ToAddress().(*iotago.AliasAddress)},
		},
		Features:          nil,
		ImmutableFeatures: nil,
	}

	foundry.Amount = te.ProtocolParameters().RentStructure.MinRent(foundry)
	newAlias.Amount -= foundry.Amount

	// Create foundry with invalid serial number
	blockB := te.NewBlockBuilder("B").
		Parents(te.LastMilestoneParents()).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput}, iotago.Outputs{foundry, newAlias}, []*utils.HDWallet{seed1Wallet}).
		Store()

	require.Equal(t, 0, len(te.UnspentFoundryOutputsInLedger()))

	// Confirming milestone at block B
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // B + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockB.StoredBlockID(), storage.ConflictInvalidChainStateTransition)
}

func TestWhiteFlagFoundryOutputInvalidAliasFoundryCounter(t *testing.T) {
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()

	// Create Alias
	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		Amount(1_000_000_000).
		BuildAlias().
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()

	require.Equal(t, 0, len(te.UnspentAliasOutputsInLedger()))

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(te.UnspentAliasOutputsInLedger()))
	aliasOutput := te.UnspentAliasOutputsInLedger()[0]
	require.Equal(t, uint32(0), aliasOutput.Output().(*iotago.AliasOutput).StateIndex)

	newAlias := aliasOutput.Output().Clone().(*iotago.AliasOutput)
	if newAlias.AliasID.Empty() {
		newAlias.AliasID = iotago.AliasIDFromOutputID(aliasOutput.OutputID())
	}

	newAlias.StateIndex++
	newAlias.FoundryCounter = 1337

	foundry := &iotago.FoundryOutput{
		Amount:       0,
		NativeTokens: nil,
		SerialNumber: 1337,
		TokenScheme: &iotago.SimpleTokenScheme{
			MintedTokens:  big.NewInt(0),
			MeltedTokens:  big.NewInt(0),
			MaximumSupply: big.NewInt(1000),
		},
		Conditions: iotago.UnlockConditions{
			&iotago.ImmutableAliasUnlockCondition{Address: newAlias.AliasID.ToAddress().(*iotago.AliasAddress)},
		},
		Features:          nil,
		ImmutableFeatures: nil,
	}

	foundry.Amount = te.ProtocolParameters().RentStructure.MinRent(foundry)
	newAlias.Amount -= foundry.Amount

	// Create foundry with invalid serial number
	blockB := te.NewBlockBuilder("B").
		Parents(te.LastMilestoneParents()).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{aliasOutput}, iotago.Outputs{foundry, newAlias}, []*utils.HDWallet{seed1Wallet}).
		Store()

	require.Equal(t, 0, len(te.UnspentFoundryOutputsInLedger()))

	// Confirming milestone at block B
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // B + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockB.StoredBlockID(), storage.ConflictInvalidChainStateTransition)
}

func TestWhiteFlagNFTOutputs(t *testing.T) {
	seed1Wallet := utils.NewHDWallet("Seed1", seed1, 0)
	seed2Wallet := utils.NewHDWallet("Seed2", seed2, 0)
	seed3Wallet := utils.NewHDWallet("Seed3", seed3, 0)

	genesisAddress := seed1Wallet.Address()

	te := testsuite.SetupTestEnvironment(t, genesisAddress, 2, ProtocolVersion, BelowMaxDepth, MinPoWScore, ShowConfirmationGraphs)
	defer te.CleanupTestEnvironment(!ShowConfirmationGraphs)

	// Add token supply to our local HDWallet
	seed1Wallet.BookOutput(te.GenesisOutput)
	te.AssertWalletBalance(seed1Wallet, te.ProtocolParameters().TokenSupply)

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	// --- Create NFT ---

	newNFT := &iotago.NFTOutput{
		Amount:       1_000_000_000,
		NativeTokens: nil,
		NFTID:        iotago.NFTID{},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: seed2Wallet.Address()},
		},
		Features: iotago.Features{
			&iotago.SenderFeature{Address: seed1Wallet.Address()},
		},
		ImmutableFeatures: iotago.Features{
			&iotago.IssuerFeature{Address: seed1Wallet.Address()},
			&iotago.MetadataFeature{Data: []byte("test")},
		},
	}

	blockA := te.NewBlockBuilder("A").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed1Wallet).
		ToWallet(seed2Wallet).
		Amount(1_000_000_000).
		BuildTransactionSendingOutputsAndCalculateRemainder(newNFT).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()

	require.Equal(t, 0, len(te.UnspentNFTOutputsInLedger()))

	// Confirming milestone at block A
	_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockA.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // A + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(te.UnspentNFTOutputsInLedger()))
	nftOutput := te.UnspentNFTOutputsInLedger()[0]
	require.Equal(t, []byte("test"), nftOutput.Output().(*iotago.NFTOutput).ImmutableFeatures.MustSet().MetadataFeature().Data)

	// --- Send NFT ---

	newNFT = nftOutput.Output().Clone().(*iotago.NFTOutput)
	newNFT.NFTID = iotago.NFTIDFromOutputID(nftOutput.OutputID())
	newNFT.Features = nil // remove previous Sender field
	newNFT.Conditions = iotago.UnlockConditions{
		&iotago.AddressUnlockCondition{Address: seed3Wallet.Address()},
	}

	blockB := te.NewBlockBuilder("B").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed2Wallet).
		ToWallet(seed3Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{nftOutput}, iotago.Outputs{newNFT}, []*utils.HDWallet{seed2Wallet}).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	require.Equal(t, 1, len(te.UnspentNFTOutputsInLedger()))

	// Confirming milestone at block B
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockB.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // B + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 1, len(te.UnspentNFTOutputsInLedger()))
	nftOutput = te.UnspentNFTOutputsInLedger()[0]
	require.Equal(t, []byte("test"), nftOutput.Output().(*iotago.NFTOutput).ImmutableFeatures.MustSet().MetadataFeature().Data)

	// --- Mutate NFT (invalid) ---

	newNFT = nftOutput.Output().Clone().(*iotago.NFTOutput)
	newNFT.Features = nil // remove previous Sender field
	newNFT.ImmutableFeatures = iotago.Features{
		newNFT.ImmutableFeatures.MustSet().IssuerFeature(),
		&iotago.MetadataFeature{Data: []byte("faked")},
	}

	blockC := te.NewBlockBuilder("B").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed3Wallet).
		ToWallet(seed3Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{nftOutput}, iotago.Outputs{newNFT}, []*utils.HDWallet{seed3Wallet}).
		Store()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	require.Equal(t, 1, len(te.UnspentNFTOutputsInLedger()))

	// Confirming milestone at block C
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockC.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // C + previous milestone
	require.Equal(t, 0, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	// Verify the blocks have the expected conflict reason
	te.AssertBlockConflictReason(blockC.StoredBlockID(), storage.ConflictInvalidChainStateTransition)

	require.Equal(t, 1, len(te.UnspentNFTOutputsInLedger()))
	nftOutput = te.UnspentNFTOutputsInLedger()[0]
	require.Equal(t, []byte("test"), nftOutput.Output().(*iotago.NFTOutput).ImmutableFeatures.MustSet().MetadataFeature().Data)

	// --- Burn NFT ---

	newBasicOutput := &iotago.BasicOutput{
		Amount:       nftOutput.Deposit(),
		NativeTokens: nil,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: seed3Wallet.Address()},
		},
		Features: nil,
	}

	blockD := te.NewBlockBuilder("D").
		Parents(te.LastMilestoneParents()).
		FromWallet(seed3Wallet).
		ToWallet(seed3Wallet).
		BuildTransactionWithInputsAndOutputs(utxo.Outputs{nftOutput}, iotago.Outputs{newBasicOutput}, []*utils.HDWallet{seed3Wallet}).
		Store().
		BookOnWallets()

	seed1Wallet.PrintStatus()
	seed2Wallet.PrintStatus()
	seed3Wallet.PrintStatus()

	require.Equal(t, 1, len(te.UnspentNFTOutputsInLedger()))

	// Confirming milestone at block D
	_, confStats = te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{blockD.StoredBlockID()}, true)
	require.Equal(t, 1+1, confStats.BlocksReferenced) // D + previous milestone
	require.Equal(t, 1, confStats.BlocksIncludedWithTransactions)
	require.Equal(t, 0, confStats.BlocksExcludedWithConflictingTransactions)
	require.Equal(t, 1, confStats.BlocksExcludedWithoutTransactions) // previous milestone

	require.Equal(t, 0, len(te.UnspentNFTOutputsInLedger()))

	te.AssertTotalSupplyStillValid()
}
