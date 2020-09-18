package testsuite

import (
	"bytes"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/muxxer/iota.go/pow"
	"github.com/muxxer/iota.go/transaction"
	"github.com/muxxer/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// storeTransaction adds the transaction to the storage layer.
func (te *TestEnvironment) storeTransaction(tx *transaction.Message) *tangle.CachedMessage {

	txTrits, err := transaction.TransactionToTrits(tx)
	require.NoError(te.testState, err)

	//txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(txTrits))
	//hornetTx := hornet.NewTransactionFromTx(tx, txBytesTruncated)

	cachedMsg, alreadyAdded := tangle.AddMessageToStorage(hornetTx, tangle.GetLatestMilestoneIndex(), false, true, true)
	require.NotNil(te.testState, cachedMsg)
	require.False(te.testState, alreadyAdded)

	return cachedMsg
}

// StoreBundle adds all messages of the bundle to the storage layer and solidifies them.
func (te *TestEnvironment) StoreMessage(msg *tangle.Message, isMilestone bool) *tangle.CachedMessage {

	var tailTx hornet.Hash
	var hashes hornet.Hashes

	// Store all messages in the database
	for i := 0; i < len(bndl); i++ {
		cachedMsg := te.storeTransaction(&bndl[i])
		require.NotNil(te.testState, cachedMsg)

		hashes = append(hashes, cachedMsg.GetMessage().GetMessageID())
		cachedMsg.Release(true)
	}

	// Solidify tx if not a milestone
	for _, hash := range hashes {
		cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(hash)
		require.NotNil(te.testState, cachedMsgMeta)

		if cachedMsgMeta.GetMetadata().IsTail() {
			tailTx = cachedMsgMeta.GetMetadata().GetMessageID()
		}

		if !isMilestone {
			cachedMsgMeta.GetMetadata().SetSolid(true)
		}

		cachedMsgMeta.Release(true)
	}

	// Trigger bundle construction due to solid tail
	if !isMilestone {
		cachedMsg := tangle.GetCachedMessageOrNil(tailTx)
		require.NotNil(te.testState, cachedMsg)
		require.True(te.testState, cachedMsg.GetMetadata().IsSolid())

		//tangle.OnMessageSolid(cachedMsg.Retain())
		cachedMsg.Release(true)
	}

	cachedMessage := tangle.GetCachedMessageOrNil(tailTx)
	require.NotNil(te.testState, cachedMessage)
	require.True(te.testState, cachedMessage.GetMessage().IsValid())
	require.True(te.testState, cachedMessage.GetMessage().ValidStrictSemantics())

	// Verify the bundle is solid if it is no milestone
	if !isMilestone {
		require.True(te.testState, cachedMessage.GetMessage().IsSolid())
	}

	te.cachedMessages = append(te.cachedMessages, cachedMessage)
	return cachedMessage
}

// AttachAndStoreBundle attaches the given bundle to the given trunk and branch and does the "Proof of Work" and stores it.
func (te *TestEnvironment) AttachAndStoreBundle(trunk hornet.Hash, branch hornet.Hash, trytes []trinary.Trytes) *tangle.CachedMessage {

	_, powFunc := pow.GetFastestProofOfWorkImpl()
	powed, err := pow.DoPoW(trunk.Hex(), branch.Hex(), trytes, mwm, powFunc)
	require.NoError(te.testState, err)

	txs, err := transaction.AsTransactionObjects(powed, nil)
	require.NoError(te.testState, err)

	return te.StoreBundle(txs, false)
}

// VerifyLSMI checks if the latest solid milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyLSMI(index milestone.Index) {
	lsmi := tangle.GetSolidMilestoneIndex()
	require.Equal(te.testState, index, lsmi)
}

// VerifyLMI checks if the latest milestone index is equal to the given milestone index.
func (te *TestEnvironment) VerifyLMI(index milestone.Index) {
	lmi := tangle.GetLatestMilestoneIndex()
	require.Equal(te.testState, index, lmi)
}

// AssertAddressBalance generates an address for the given seed and index and checks correct balance.
func (te *TestEnvironment) AssertAddressBalance(seed trinary.Trytes, index uint64, balance uint64) {
	address := utils.GenerateAddress(te.testState, seed, index)
	addrBalance, _, err := tangle.GetBalanceForAddress(address)
	require.NoError(te.testState, err)
	require.Equal(te.testState, balance, addrBalance)
}

// AssertTotalSupplyStillValid checks if the total supply in the database is still correct.
func (te *TestEnvironment) AssertTotalSupplyStillValid() {
	_, _, err := tangle.GetLedgerStateForLSMI(nil)
	require.NoError(te.testState, err)
}

// generateDotFileFromConfirmation generates a dot file from a whiteflag confirmation cone.
func (te *TestEnvironment) generateDotFileFromConfirmation(conf *whiteflag.Confirmation) string {

	indexOf := func(hash hornet.Hash) int {
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

	visitedCachedMessages := make(map[string]tangle.CachedMessages)

	bundleTxs := tangle.GetAllBundleTransactionHashes(100)
	for _, hash := range bundleTxs {
		cachedMsgMeta := tangle.GetCachedMessageMetadataOrNil(hash)
		if _, visited := visitedCachedMessages[string(cachedMsgMeta.GetMetadata().GetBundleHash())]; !visited {
			bndls := tangle.GetBundlesOfTransactionOrNil(cachedMsgMeta.GetMetadata().GetMessageID(), false)
			visitedCachedMessages[string(cachedMsgMeta.GetMetadata().GetBundleHash())] = bndls
		}
		cachedMsgMeta.Release(true)
	}

	var milestones []string
	var included []string
	var ignored []string
	var conflicting []string

	dotFile := fmt.Sprintf("digraph %s\n{\n", te.testState.Name())
	for _, cachedMessage := range visitedCachedMessages {
		//singleBundle := len(bndls) == 1
		for _, bndl := range cachedMessage {
			shortBundle := utils.ShortenedTag(bndl)

			tailHash := bndl.GetMessage().GetMessageID()
			if index := indexOf(tailHash); index != -1 {
				dotFile += fmt.Sprintf("\"%s\" [ label=\"[%d] %s\" ];\n", shortBundle, index, shortBundle)
			}

			trunk := bndl.GetMessage().GetParent1MessageID(true)
			if tangle.SolidEntryPointsContain(trunk) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Trunk\" ];\n", shortBundle, utils.ShortenedHash(trunk))
			} else {
				trunkBundles := tangle.GetBundlesOfTransactionOrNil(trunk, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Trunk\" ];\n", shortBundle, utils.ShortenedTag(trunkBundles[0]))
				trunkBundles.Release()
			}

			branch := bndl.GetMessage().GetParent2MessageID(true)
			if tangle.SolidEntryPointsContain(branch) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Branch\" ];\n", shortBundle, utils.ShortenedHash(branch))
			} else {
				branchBundles := tangle.GetBundlesOfTransactionOrNil(branch, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Branch\" ];\n", shortBundle, utils.ShortenedTag(branchBundles[0]))
				branchBundles.Release()
			}

			if bndl.GetMessage().IsMilestone() {
				if conf != nil && bndl.GetMessage().GetMilestoneIndex() == conf.MilestoneIndex {
					dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gold];\n", shortBundle)
				}
				milestones = append(milestones, shortBundle)
			} else if bndl.GetMessage().IsConfirmed() {
				if bndl.GetMessage().IsConflicting() {
					conflicting = append(conflicting, shortBundle)
				} else if bndl.GetMessage().IsValueSpam() {
					ignored = append(ignored, shortBundle)
				} else {
					included = append(included, shortBundle)
				}
			}
		}
		cachedMessage.Release()
	}

	for _, milestone := range milestones {
		dotFile += fmt.Sprintf("\"%s\" [shape=Msquare];\n", milestone)
	}
	for _, conf := range conflicting {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=red];\n", conf)
	}
	for _, conf := range ignored {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gray];\n", conf)
	}
	for _, conf := range included {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=green];\n", conf)
	}

	dotFile += "}\n"
	return dotFile
}
