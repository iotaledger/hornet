package testsuite

import (
	"bytes"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/compressed"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// storeTransaction adds the transaction to the storage layer.
func (te *TestEnvironment) storeTransaction(tx *transaction.Transaction) *tangle.CachedTransaction {

	txTrits, err := transaction.TransactionToTrits(tx)
	require.NoError(te.testState, err)

	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(txTrits))
	hornetTx := hornet.NewTransactionFromTx(tx, txBytesTruncated)

	cachedTx, alreadyAdded := tangle.AddTransactionToStorage(hornetTx, tangle.GetLatestMilestoneIndex(), false, true, true)
	require.NotNil(te.testState, cachedTx)
	require.False(te.testState, alreadyAdded)

	return cachedTx
}

// StoreBundle adds all transactions of the bundle to the storage layer and solidifies them.
func (te *TestEnvironment) StoreBundle(bndl bundle.Bundle, isMilestone bool) *tangle.CachedBundle {

	var tailTx hornet.Hash
	var hashes hornet.Hashes

	// Store all transactions in the database
	for i := 0; i < len(bndl); i++ {
		cachedTx := te.storeTransaction(&bndl[i])
		require.NotNil(te.testState, cachedTx)

		hashes = append(hashes, cachedTx.GetTransaction().GetTxHash())
		cachedTx.Release(true)
	}

	// Solidify tx if not a milestone
	for _, hash := range hashes {
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hash)
		require.NotNil(te.testState, cachedTxMeta)

		if cachedTxMeta.GetMetadata().IsTail() {
			tailTx = cachedTxMeta.GetMetadata().GetTxHash()
		}

		if !isMilestone {
			cachedTxMeta.GetMetadata().SetSolid(true)
		}

		cachedTxMeta.Release(true)
	}

	// Trigger bundle construction due to solid tail
	if !isMilestone {
		cachedTx := tangle.GetCachedTransactionOrNil(tailTx)
		require.NotNil(te.testState, cachedTx)
		require.True(te.testState, cachedTx.GetMetadata().IsSolid())

		tangle.OnTailTransactionSolid(cachedTx.Retain())
		cachedTx.Release(true)
	}

	cachedBundle := tangle.GetCachedBundleOrNil(tailTx)
	require.NotNil(te.testState, cachedBundle)
	require.True(te.testState, cachedBundle.GetBundle().IsValid())
	require.True(te.testState, cachedBundle.GetBundle().ValidStrictSemantics())

	// Verify the bundle is solid if it is no milestone
	if !isMilestone {
		require.True(te.testState, cachedBundle.GetBundle().IsSolid())
	}

	te.cachedBundles = append(te.cachedBundles, cachedBundle)
	return cachedBundle
}

// AttachAndStoreBundle attaches the given bundle to the given trunk and branch and does the "Proof of Work" and stores it.
func (te *TestEnvironment) AttachAndStoreBundle(trunk hornet.Hash, branch hornet.Hash, trytes []trinary.Trytes) *tangle.CachedBundle {

	_, powFunc := pow.GetFastestProofOfWorkImpl()
	powed, err := pow.DoPoW(trunk.Trytes(), branch.Trytes(), trytes, mwm, powFunc)
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
		for i := 0; i < len(conf.Mutations.TailsReferenced)-1; i++ {
			if bytes.Equal(conf.Mutations.TailsReferenced[i], hash) {
				return i
			}
		}
		return -1
	}

	visitedBundles := make(map[string]tangle.CachedBundles)

	bundleTxs := tangle.GetAllBundleTransactionHashes(100)
	for _, hash := range bundleTxs {
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hash)
		if _, visited := visitedBundles[string(cachedTxMeta.GetMetadata().GetBundleHash())]; !visited {
			bndls := tangle.GetBundlesOfTransactionOrNil(cachedTxMeta.GetMetadata().GetTxHash(), false)
			visitedBundles[string(cachedTxMeta.GetMetadata().GetBundleHash())] = bndls
		}
		cachedTxMeta.Release(true)
	}

	var milestones []string
	var included []string
	var ignored []string
	var conflicting []string

	dotFile := fmt.Sprintf("digraph %s\n{\n", te.testState.Name())
	for _, bndls := range visitedBundles {
		//singleBundle := len(bndls) == 1
		for _, bndl := range bndls {
			shortBundle := utils.ShortenedTag(bndl)

			tailHash := bndl.GetBundle().GetTailHash()
			if index := indexOf(tailHash); index != -1 {
				dotFile += fmt.Sprintf("\"%s\" [ label=\"[%d] %s\" ];\n", shortBundle, index, shortBundle)
			}

			trunk := bndl.GetBundle().GetTrunkHash(true)
			if tangle.SolidEntryPointsContain(trunk) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Trunk\" ];\n", shortBundle, utils.ShortenedHash(trunk))
			} else {
				trunkBundles := tangle.GetBundlesOfTransactionOrNil(trunk, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Trunk\" ];\n", shortBundle, utils.ShortenedTag(trunkBundles[0]))
				trunkBundles.Release()
			}

			branch := bndl.GetBundle().GetBranchHash(true)
			if tangle.SolidEntryPointsContain(branch) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Branch\" ];\n", shortBundle, utils.ShortenedHash(branch))
			} else {
				branchBundles := tangle.GetBundlesOfTransactionOrNil(branch, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Branch\" ];\n", shortBundle, utils.ShortenedTag(branchBundles[0]))
				branchBundles.Release()
			}

			if bndl.GetBundle().IsMilestone() {
				if conf != nil && bndl.GetBundle().GetMilestoneIndex() == conf.MilestoneIndex {
					dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gold];\n", shortBundle)
				}
				milestones = append(milestones, shortBundle)
			} else if bndl.GetBundle().IsConfirmed() {
				if bndl.GetBundle().IsConflicting() {
					conflicting = append(conflicting, shortBundle)
				} else if bndl.GetBundle().IsValueSpam() {
					ignored = append(ignored, shortBundle)
				} else {
					included = append(included, shortBundle)
				}
			}
		}
		bndls.Release()
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
