package testsuite

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/pkg/testsuite/utils"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

// StoreBlock adds the block to the storage layer and solidifies it.
// block +1.
func (te *TestEnvironment) StoreBlock(block *storage.Block) *storage.CachedBlock {

	// Store block in the database
	cachedBlock, alreadyAdded := tangle.AddBlockToStorage(te.storage, te.milestoneManager, block, te.syncManager.LatestMilestoneIndex(), false, true)
	require.NotNil(te.TestInterface, cachedBlock)
	require.False(te.TestInterface, alreadyAdded)

	// Solidify block
	cachedBlock.Metadata().SetSolid(true)
	require.True(te.TestInterface, cachedBlock.Metadata().IsSolid())

	te.cachedBlocks = append(te.cachedBlocks, cachedBlock)

	return cachedBlock
}

// VerifyCMI checks if the confirmed milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyCMI(index iotago.MilestoneIndex) {
	cmi := te.syncManager.ConfirmedMilestoneIndex()
	require.Equal(te.TestInterface, index, cmi)
}

// VerifyLMI checks if the latest milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyLMI(index iotago.MilestoneIndex) {
	lmi := te.syncManager.LatestMilestoneIndex()
	require.Equal(te.TestInterface, index, lmi)
}

// AssertLedgerBalance generates an address for the given seed and index and checks correct balance.
func (te *TestEnvironment) AssertLedgerBalance(wallet *utils.HDWallet, expectedBalance uint64) {
	computedAddrBalance, outputCount, err := te.ComputeAddressBalanceWithoutConstraints(wallet.Address())
	require.NoError(te.TestInterface, err)

	var balanceStatus string
	balanceStatus += fmt.Sprintf("Balance for %s:\n", wallet.Name())
	balanceStatus += fmt.Sprintf("\tComputed:\t%d\n", computedAddrBalance)
	balanceStatus += fmt.Sprintf("\tExpected:\t%d\n", expectedBalance)
	balanceStatus += fmt.Sprintf("\tOutputCount:\t%d\n", outputCount)
	fmt.Print(balanceStatus)

	require.Exactly(te.TestInterface, expectedBalance, computedAddrBalance)
}

// AssertWalletBalance generates an address for the given seed and index and checks correct balance.
func (te *TestEnvironment) AssertWalletBalance(wallet *utils.HDWallet, expectedBalance uint64) {
	computedAddrBalance, _, err := te.ComputeAddressBalanceWithoutConstraints(wallet.Address())
	require.NoError(te.TestInterface, err)

	var balanceStatus string
	balanceStatus += fmt.Sprintf("Balance for %s:\n", wallet.Name())
	balanceStatus += fmt.Sprintf("\tComputed:\t%d\n", computedAddrBalance)
	balanceStatus += fmt.Sprintf("\tWallet:\t\t%d\n", wallet.Balance())
	balanceStatus += fmt.Sprintf("\tExpected:\t%d\n", expectedBalance)
	fmt.Print(balanceStatus)

	require.Exactly(te.TestInterface, expectedBalance, computedAddrBalance)
	require.Exactly(te.TestInterface, expectedBalance, wallet.Balance())
}

// AssertTotalSupplyStillValid checks if the total supply in the database is still correct.
func (te *TestEnvironment) AssertTotalSupplyStillValid() {
	err := te.storage.CheckLedgerState()
	require.NoError(te.TestInterface, err)
}

func (te *TestEnvironment) AssertBlockConflictReason(blockID iotago.BlockID, conflict storage.Conflict) {
	cachedBlockMeta := te.storage.CachedBlockMetadataOrNil(blockID)
	require.NotNil(te.TestInterface, cachedBlockMeta)
	defer cachedBlockMeta.Release(true) // meta -1
	require.Equal(te.TestInterface, conflict, cachedBlockMeta.Metadata().Conflict())
}

// generateDotFileFromConfirmation generates a dot file from a whiteflag confirmation cone.
func (te *TestEnvironment) generateDotFileFromConfirmation(conf *whiteflag.Confirmation) string {

	indexOf := func(blockID iotago.BlockID) int {
		if conf == nil {
			return -1
		}

		for i := 0; i < len(conf.Mutations.ReferencedBlocks)-1; i++ {
			if conf.Mutations.ReferencedBlocks[i].BlockID == blockID {
				return i
			}
		}

		return -1
	}

	visitedCachedBlocks := make(map[iotago.BlockID]*storage.CachedBlock)

	milestoneParents, err := te.storage.MilestoneParentsByIndex(conf.MilestoneIndex)
	require.NoError(te.TestInterface, err, "milestone doesn't exist (%d)", conf.MilestoneIndex)

	err = dag.TraverseParents(
		context.Background(),
		te.storage,
		milestoneParents,
		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			return true, nil
		},
		// consumer
		func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			if _, visited := visitedCachedBlocks[cachedBlockMeta.Metadata().BlockID()]; !visited {
				cachedBlock := te.storage.CachedBlockOrNil(cachedBlockMeta.Metadata().BlockID()) // block +1
				require.NotNil(te.TestInterface, cachedBlock)
				visitedCachedBlocks[cachedBlockMeta.Metadata().BlockID()] = cachedBlock
			}

			return nil
		},
		// called on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false)
	require.NoError(te.TestInterface, err)

	var milestoneBlocks []string
	var includedBlocks []string
	var ignoredBlocks []string
	var conflictingBlocks []string

	dotFile := fmt.Sprintf("digraph %s\n{\n", te.TestInterface.Name())
	for _, cachedBlock := range visitedCachedBlocks {
		block := cachedBlock.Block()
		meta := cachedBlock.Metadata()

		shortIndex := utils.ShortenedTag(cachedBlock.Retain()) // block pass +1

		if index := indexOf(block.BlockID()); index != -1 {
			dotFile += fmt.Sprintf("\"%s\" [ label=\"[%d] %s\" ];\n", shortIndex, index, shortIndex)
		}

		for i, parent := range block.Parents() {
			contains, err := te.storage.SolidEntryPointsContain(parent)
			if err != nil {
				panic(err)
			}
			if contains {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Parent%d\" ];\n", shortIndex, utils.ShortenedHash(parent), i+1)

				continue
			}

			cachedBlockParent := te.storage.CachedBlockOrNil(parent)
			require.NotNil(te.TestInterface, cachedBlockParent)
			dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Parent%d\" ];\n", shortIndex, utils.ShortenedTag(cachedBlockParent.Retain()), i+1) // block pass +1
			cachedBlockParent.Release(true)                                                                                                       // block -1
		}

		milestonePayload := block.Milestone()
		if milestonePayload != nil {
			if conf != nil && milestonePayload.Index == conf.MilestoneIndex {
				dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gold];\n", shortIndex)
			}
			milestoneBlocks = append(milestoneBlocks, shortIndex)
		} else if meta.IsReferenced() {
			if meta.IsConflictingTx() {
				conflictingBlocks = append(conflictingBlocks, shortIndex)
			} else if meta.IsNoTransaction() {
				ignoredBlocks = append(ignoredBlocks, shortIndex)
			} else if meta.IsIncludedTxInLedger() {
				includedBlocks = append(includedBlocks, shortIndex)
			} else {
				panic("unknown block state")
			}
		}
		cachedBlock.Release(true) // block -1
	}

	for _, milestoneBlock := range milestoneBlocks {
		dotFile += fmt.Sprintf("\"%s\" [shape=Msquare];\n", milestoneBlock)
	}
	for _, conflictingBlock := range conflictingBlocks {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=red];\n", conflictingBlock)
	}
	for _, ignoredBlock := range ignoredBlocks {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gray];\n", ignoredBlock)
	}
	for _, includedBlock := range includedBlocks {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=green];\n", includedBlock)
	}

	dotFile += "}\n"

	return dotFile
}
