package testsuite

import (
	"bytes"
	"context"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// StoreMessage adds the message to the storage layer and solidifies it.
// message +1
func (te *TestEnvironment) StoreMessage(msg *storage.Message) *storage.CachedMessage {

	// Store message in the database
	cachedBlock, alreadyAdded := tangle.AddMessageToStorage(te.storage, te.milestoneManager, msg, te.syncManager.LatestMilestoneIndex(), false, true)
	require.NotNil(te.TestInterface, cachedBlock)
	require.False(te.TestInterface, alreadyAdded)

	// Solidify msg
	cachedBlock.Metadata().SetSolid(true)
	require.True(te.TestInterface, cachedBlock.Metadata().IsSolid())

	te.cachedMessages = append(te.cachedMessages, cachedBlock)

	return cachedBlock
}

// VerifyCMI checks if the confirmed milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyCMI(index milestone.Index) {
	cmi := te.syncManager.ConfirmedMilestoneIndex()
	require.Equal(te.TestInterface, index, cmi)
}

// VerifyLMI checks if the latest milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyLMI(index milestone.Index) {
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
	err := te.storage.UTXOManager().CheckLedgerState(te.protoParas)
	require.NoError(te.TestInterface, err)
}

func (te *TestEnvironment) AssertMessageConflictReason(blockID hornet.BlockID, conflict storage.Conflict) {
	cachedBlockMeta := te.storage.CachedMessageMetadataOrNil(blockID)
	require.NotNil(te.TestInterface, cachedBlockMeta)
	defer cachedBlockMeta.Release(true) // meta -1
	require.Equal(te.TestInterface, cachedBlockMeta.Metadata().Conflict(), conflict)
}

// generateDotFileFromConfirmation generates a dot file from a whiteflag confirmation cone.
func (te *TestEnvironment) generateDotFileFromConfirmation(conf *whiteflag.Confirmation) string {

	indexOf := func(hash hornet.BlockID) int {
		if conf == nil {
			return -1
		}

		for i := 0; i < len(conf.Mutations.MessagesReferenced)-1; i++ {
			if bytes.Equal(conf.Mutations.MessagesReferenced[i], hash) {
				return i
			}
		}

		return -1
	}

	visitedCachedMessages := make(map[string]*storage.CachedMessage)

	milestoneParents, err := te.storage.MilestoneParentsByIndex(conf.MilestoneIndex)
	require.NoError(te.TestInterface, err, "milestone doesn't exist (%d)", conf.MilestoneIndex)

	err = dag.TraverseParents(
		context.Background(),
		te.storage,
		milestoneParents,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1
			return true, nil
		},
		// consumer
		func(cachedBlockMeta *storage.CachedMetadata) error { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			if _, visited := visitedCachedMessages[cachedBlockMeta.Metadata().MessageID().ToMapKey()]; !visited {
				cachedBlock := te.storage.CachedMessageOrNil(cachedBlockMeta.Metadata().MessageID()) // message +1
				require.NotNil(te.TestInterface, cachedBlock)
				visitedCachedMessages[cachedBlockMeta.Metadata().MessageID().ToMapKey()] = cachedBlock
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

	var milestoneMsgs []string
	var includedMsgs []string
	var ignoredMsgs []string
	var conflictingMsgs []string

	dotFile := fmt.Sprintf("digraph %s\n{\n", te.TestInterface.Name())
	for _, cachedBlock := range visitedCachedMessages {
		message := cachedBlock.Message()
		meta := cachedBlock.Metadata()

		shortIndex := utils.ShortenedTag(cachedBlock.Retain()) // message pass +1

		if index := indexOf(message.MessageID()); index != -1 {
			dotFile += fmt.Sprintf("\"%s\" [ label=\"[%d] %s\" ];\n", shortIndex, index, shortIndex)
		}

		for i, parent := range message.Parents() {
			contains, err := te.storage.SolidEntryPointsContain(parent)
			if err != nil {
				panic(err)
			}
			if contains {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Parent%d\" ];\n", shortIndex, utils.ShortenedHash(parent), i+1)
				continue
			}

			cachedBlockParent := te.storage.CachedMessageOrNil(parent)
			require.NotNil(te.TestInterface, cachedBlockParent)
			dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Parent%d\" ];\n", shortIndex, utils.ShortenedTag(cachedBlockParent.Retain()), i+1) // message pass +1
			cachedBlockParent.Release(true)                                                                                                       // message -1
		}

		milestonePayload := message.Milestone()
		if milestonePayload != nil {
			if conf != nil && milestone.Index(milestonePayload.Index) == conf.MilestoneIndex {
				dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gold];\n", shortIndex)
			}
			milestoneMsgs = append(milestoneMsgs, shortIndex)
		} else if meta.IsReferenced() {
			if meta.IsConflictingTx() {
				conflictingMsgs = append(conflictingMsgs, shortIndex)
			} else if meta.IsNoTransaction() {
				ignoredMsgs = append(ignoredMsgs, shortIndex)
			} else if meta.IsIncludedTxInLedger() {
				includedMsgs = append(includedMsgs, shortIndex)
			} else {
				panic("unknown msg state")
			}
		}
		cachedBlock.Release(true) // message -1
	}

	for _, milestone := range milestoneMsgs {
		dotFile += fmt.Sprintf("\"%s\" [shape=Msquare];\n", milestone)
	}
	for _, conflictingMsg := range conflictingMsgs {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=red];\n", conflictingMsg)
	}
	for _, ignoredMsg := range ignoredMsgs {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gray];\n", ignoredMsg)
	}
	for _, includedMsg := range includedMsgs {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=green];\n", includedMsg)
	}

	dotFile += "}\n"
	return dotFile
}
