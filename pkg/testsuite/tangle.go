package testsuite

import (
	"bytes"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// StoreMessage adds the message to the storage layer and solidifies it.
func (te *TestEnvironment) StoreMessage(msg *storage.Message) *storage.CachedMessage {

	// Store message in the database
	cachedMsg, alreadyAdded := te.storage.AddMessageToStorage(msg, te.storage.GetLatestMilestoneIndex(), false, true, true)
	require.NotNil(te.TestState, cachedMsg)
	require.False(te.TestState, alreadyAdded)

	// Solidify msg if not a milestone
	ms := msg.GetMilestone()
	if ms == nil {
		cachedMsg.GetMetadata().SetSolid(true)
		require.True(te.TestState, cachedMsg.GetMetadata().IsSolid())
	}

	te.cachedMessages = append(te.cachedMessages, cachedMsg)

	return cachedMsg
}

// VerifyCMI checks if the confirmed milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyCMI(index milestone.Index) {
	cmi := te.storage.GetConfirmedMilestoneIndex()
	require.Equal(te.TestState, index, cmi)
}

// VerifyLMI checks if the latest milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyLMI(index milestone.Index) {
	lmi := te.storage.GetLatestMilestoneIndex()
	require.Equal(te.TestState, index, lmi)
}

// AssertAddressBalance generates an address for the given seed and index and checks correct balance.
func (te *TestEnvironment) AssertWalletBalance(wallet *utils.HDWallet, expectedBalance uint64) {
	addrBalance, _, err := te.storage.UTXO().AddressBalance(wallet.Address())
	require.NoError(te.TestState, err)
	computedAddrBalance, _, err := te.storage.UTXO().ComputeBalance(utxo.FilterAddress(wallet.Address()))
	require.NoError(te.TestState, err)

	var balanceStatus string
	balanceStatus += fmt.Sprintf("Balance for %s:\n", wallet.Name())
	balanceStatus += fmt.Sprintf("\tLedger:\t\t%d\n", computedAddrBalance)
	balanceStatus += fmt.Sprintf("\tComputed:\t%d\n", computedAddrBalance)
	balanceStatus += fmt.Sprintf("\tWallet:\t\t%d\n", wallet.Balance())
	balanceStatus += fmt.Sprintf("\tExpected:\t%d\n", expectedBalance)
	fmt.Print(balanceStatus)

	require.Exactly(te.TestState, expectedBalance, addrBalance)
	require.Exactly(te.TestState, expectedBalance, computedAddrBalance)
	require.Exactly(te.TestState, expectedBalance, wallet.Balance())
}

// AssertTotalSupplyStillValid checks if the total supply in the database is still correct.
func (te *TestEnvironment) AssertTotalSupplyStillValid() {
	err := te.storage.UTXO().CheckLedgerState()
	require.NoError(te.TestState, err)
}

func (te *TestEnvironment) AssertMessageConflictReason(messageID hornet.MessageID, conflict storage.Conflict) {
	cachedMsgMeta := te.storage.GetCachedMessageMetadataOrNil(messageID)
	require.NotNil(te.TestState, cachedMsgMeta)
	defer cachedMsgMeta.Release(true)
	require.Equal(te.TestState, cachedMsgMeta.GetMetadata().GetConflict(), conflict)
}

// generateDotFileFromConfirmation generates a dot file from a whiteflag confirmation cone.
func (te *TestEnvironment) generateDotFileFromConfirmation(conf *whiteflag.Confirmation) string {

	indexOf := func(hash hornet.MessageID) int {
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

	err := dag.TraverseParentsOfMessage(te.storage, conf.MilestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1
			return true, nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			if _, visited := visitedCachedMessages[cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()]; !visited {
				cachedMsg := te.storage.GetCachedMessageOrNil(cachedMsgMeta.GetMetadata().GetMessageID())
				require.NotNil(te.TestState, cachedMsg)
				visitedCachedMessages[cachedMsgMeta.GetMetadata().GetMessageID().ToMapKey()] = cachedMsg
			}

			return nil
		},
		// called on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false, nil)
	require.NoError(te.TestState, err)

	var milestoneMsgs []string
	var includedMsgs []string
	var ignoredMsgs []string
	var conflictingMsgs []string

	dotFile := fmt.Sprintf("digraph %s\n{\n", te.TestState.Name())
	for _, cachedMessage := range visitedCachedMessages {
		message := cachedMessage.GetMessage()
		meta := cachedMessage.GetMetadata()

		shortIndex := utils.ShortenedIndex(cachedMessage.Retain())

		if index := indexOf(message.GetMessageID()); index != -1 {
			dotFile += fmt.Sprintf("\"%s\" [ label=\"[%d] %s\" ];\n", shortIndex, index, shortIndex)
		}

		for i, parent := range message.GetParents() {
			if te.storage.SolidEntryPointsContain(parent) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Parent%d\" ];\n", shortIndex, utils.ShortenedHash(parent), i+1)
				continue
			}

			cachedMessageParent := te.storage.GetCachedMessageOrNil(parent)
			require.NotNil(te.TestState, cachedMessageParent)
			dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Parent%d\" ];\n", shortIndex, utils.ShortenedIndex(cachedMessageParent.Retain()), i+1)
			cachedMessageParent.Release(true)
		}

		ms := message.GetMilestone()
		if ms != nil {
			if conf != nil && milestone.Index(ms.Index) == conf.MilestoneIndex {
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
		cachedMessage.Release(true)
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
